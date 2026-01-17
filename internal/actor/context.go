package actor

import (
	"fmt"
	"reflect"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/chain"
	"github.com/kercylan98/vivid/internal/future"
	"github.com/kercylan98/vivid/internal/mailbox"
	"github.com/kercylan98/vivid/internal/messages"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/metrics"
	"github.com/kercylan98/vivid/pkg/ves"
)

var (
	_ vivid.ActorContext   = (*Context)(nil)
	_ vivid.EnvelopHandler = (*Context)(nil)
)

var (
	actorIncrementId atomic.Uint64
)

const (
	running int32 = iota
	killing
	killed
)

func NewContext(system *System, parent *Ref, actor vivid.Actor, options ...vivid.ActorOption) (*Context, error) {
	ctx := &Context{
		options: &vivid.ActorOptions{
			DefaultAskTimeout: system.options.DefaultAskTimeout, // 默认继承系统默认的 Ask 超时时间
			Logger:            system.options.Logger,            // 默认继承系统默认的日志记录器
		},
		system:        system,
		parent:        parent,
		behaviorStack: NewBehaviorStack(),
	}
	ctx.scheduler = newScheduler(ctx)

	initializer := newContextInitializer(ctx, actor, options...)

	if err := chain.New().
		Append(chain.ChainFN(initializer.applyOptions)).
		Append(chain.ChainFN(initializer.initActor)).
		Append(chain.ChainFN(initializer.initRef)).
		Append(chain.ChainFN(initializer.prelaunch)).
		Append(chain.ChainFN(initializer.initMailbox)).
		Append(chain.ChainFN(initializer.initBehavior)).
		Run(); err != nil {
		return nil, err
	}
	return ctx, nil
}

type Context struct {
	options       *vivid.ActorOptions                // 当前 ActorContext 的选项
	system        *System                            // 当前 ActorContext 所属的 ActorSystem
	parent        *Ref                               // 父 Actor 引用，如果为 nil 则表示根 Actor
	ref           *Ref                               // 当前 Actor 引用
	actor         vivid.Actor                        // 当前 Actor
	behaviorStack *BehaviorStack                     // 行为栈
	mailbox       vivid.Mailbox                      // 邮箱
	children      map[vivid.ActorPath]vivid.ActorRef // 懒加载的子 Actor 引用
	envelop       vivid.Envelop                      // 当前 ActorContext 的消息
	state         int32                              // 状态
	restarting    *RestartMessage                    // 正在重启的消息
	watchers      map[string]vivid.ActorRef          // 正在监听该 Actor 终止事件的 ActorRef，其中 key 为 ActorRef 的完整路径
	stash         []vivid.Envelop                    // 暂存区
	scheduler     *Scheduler                         // 调度器
}

func (c *Context) Stash() {
	c.stash = append(c.stash, c.envelop)
}

func (c *Context) Unstash(num ...int) {
	stashCount := len(c.stash)
	if stashCount == 0 {
		return
	}

	// 快速通道
	if len(num) == 0 {
		c.mailbox.Enqueue(c.stash[0])
		c.stash = c.stash[1:]
		return
	}

	// 批量恢复
	popCount := num[0]
	if popCount <= 0 {
		popCount = stashCount
	}

	for i := 0; i < popCount; i++ {
		c.mailbox.Enqueue(c.stash[i])
	}
	c.stash = c.stash[popCount:]

	// 如果全部恢复，则释放底层数组
	if stashCount == popCount {
		c.stash = nil
	}
}

func (c *Context) StashCount() int {
	return len(c.stash)
}

func (c *Context) Metrics() metrics.Metrics {
	return c.system.Metrics()
}

func (c *Context) Logger() log.Logger {
	return c.options.Logger
}

func (c *Context) System() vivid.ActorSystem {
	return c.system
}

func (c *Context) Parent() vivid.ActorRef {
	return c.parent
}

func (c *Context) Ref() vivid.ActorRef {
	return c.ref
}

func (c *Context) Mailbox() vivid.Mailbox {
	return c.mailbox
}

func (c *Context) Name() string {
	return c.options.Name
}

func (c *Context) EventStream() vivid.EventStream {
	return c.system.eventStream
}

func (c *Context) Scheduler() vivid.Scheduler {
	return c.scheduler
}

func (c *Context) ActorOf(actor vivid.Actor, options ...vivid.ActorOption) (vivid.ActorRef, error) {
	var status = atomic.LoadInt32(&c.state)
	if status == killed {
		return nil, fmt.Errorf("actor killed")
	}

	childCtx, err := NewContext(c.system, c.ref, actor, options...)
	if err != nil {
		return nil, err
	}

	if c.system.appendActorContext(childCtx) {
		return nil, fmt.Errorf("already exists")
	}

	if c.children == nil {
		c.children = make(map[vivid.ActorPath]vivid.ActorRef)
	}
	c.children[childCtx.Ref().GetPath()] = childCtx.Ref()

	c.tell(true, childCtx.Ref(), new(vivid.OnLaunch))
	c.Logger().Debug("actor spawned", log.String("path", childCtx.Ref().GetPath()))

	// 通知事件流
	c.EventStream().Publish(c, ves.ActorSpawnedEvent{
		ActorRef: childCtx.Ref(),
		Type:     reflect.TypeOf(actor),
	})

	if status == killing {
		c.Kill(childCtx.ref, false, "parent killed")
	}
	return childCtx.Ref(), nil
}

func (c *Context) Reply(message vivid.Message) {
	c.Tell(c.envelop.Sender(), message)
}

func (c *Context) TellSelf(message vivid.Message) {
	c.mailbox.Enqueue(mailbox.NewEnvelop(false, c.ref, c.ref, message))
}

func (c *Context) Tell(recipient vivid.ActorRef, message vivid.Message) {
	c.tell(false, recipient, message)
}

func (c *Context) tell(system bool, recipient vivid.ActorRef, message vivid.Message) {
	envelop := mailbox.NewEnvelop(system, c.ref, recipient, message)
	receiverMailbox := c.system.findMailbox(recipient.(*Ref))
	receiverMailbox.Enqueue(envelop)
}

func (c *Context) Ask(recipient vivid.ActorRef, message vivid.Message, timeout ...time.Duration) vivid.Future[vivid.Message] {
	return c.ask(false, recipient, message, timeout...)
}

func (c *Context) ask(system bool, recipient vivid.ActorRef, message vivid.Message, timeout ...time.Duration) vivid.Future[vivid.Message] {
	var askTimeout = c.options.DefaultAskTimeout
	if len(timeout) > 0 {
		askTimeout = timeout[0]
	}

	agentRef, err := NewAgentRef(c.ref)
	if err != nil {
		return future.NewFutureFail[vivid.Message](err)
	}
	futureIns := future.NewFuture[vivid.Message](askTimeout, func() {
		c.system.removeFuture(agentRef)
	})
	c.system.appendFuture(agentRef, futureIns)

	envelop := mailbox.NewEnvelop(system, agentRef.ref, recipient, message).WithAgent(agentRef.agent)
	receiverMailbox := c.system.findMailbox(recipient.(*Ref))
	receiverMailbox.Enqueue(envelop)

	return futureIns
}

func (c *Context) Entrust(timeout time.Duration, task vivid.EntrustTask) vivid.Future[vivid.Message] {
	if task == nil {
		return future.NewFutureFail[vivid.Message](vivid.ErrorFutureInvalid.WithMessage("no task to be executed"))
	}

	futureIns := future.NewFuture[vivid.Message](timeout, nil)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				switch r := r.(type) {
				case error:
					futureIns.Close(vivid.ErrorFutureUnexpectedError.With(r))
				default:
					futureIns.Close(vivid.ErrorFutureUnexpectedError.With(fmt.Errorf("unexpected error: %v", r)))
				}
			}
		}()
		message, err := task.Run()
		if err != nil {
			futureIns.Close(err)
		} else {
			futureIns.EnqueueMessage(message)
		}
	}()
	return futureIns
}

func (c *Context) PipeTo(recipient vivid.ActorRef, message vivid.Message, forwarders vivid.ActorRefs, timeout ...time.Duration) string {
	pipeId := uuid.NewString()
	pipeFuture := c.ask(false, recipient, message, timeout...)

	if len(forwarders) == 0 {
		return pipeId
	}

	go func(c *Context, pipeId string, future vivid.Future[vivid.Message]) {
		var pipeResult = &vivid.PipeResult{
			Id: pipeId,
		}

		pipeResult.Message, pipeResult.Error = future.Result()
		for _, forwarder := range forwarders {
			if forwarder.Equals(c.ref) {
				c.TellSelf(pipeResult)
				continue
			}
			c.tell(false, forwarder, pipeResult)
		}

	}(c, pipeId, pipeFuture)
	return pipeId
}

func (c *Context) HandleEnvelop(envelop vivid.Envelop) {
	// 非运行状态下：
	// - 普通消息一律推入死信队列
	// - 系统消息在 killing 阶段仍需要处理（例如子 Actor 的 OnKilled 事件），否则终止流程无法闭环
	currentState := atomic.LoadInt32(&c.state)
	if (currentState == killed) || (!envelop.System() && currentState != running) {
		c.system.TellSelf(ves.DeathLetterEvent{
			Envelope: envelop,
			Time:     time.Now(),
		})
		return
	}

	// 处理消息
	c.envelop = envelop
	behavior := c.behaviorStack.Peek()

	switch message := c.envelop.Message().(type) {
	case *vivid.OnLaunch:
		behavior(c)
		// 通知事件流
		c.EventStream().Publish(c, ves.ActorLaunchedEvent{
			ActorRef: c.ref,
			Type:     reflect.TypeOf(c.actor),
		})
	case *vivid.OnKill:
		c.onKill(message, behavior)
	case *vivid.OnKilled:
		c.onKilled(message, behavior)
	case *supervisionContext:
		c.onSupervise(message)
	case *messages.NoneArgsCommandMessage:
		c.onCommand(message)
	case *RestartMessage:
		c.onRestart(message, behavior)
	case *messages.PingMessage:
		c.onPing(message)
	case *messages.WatchMessage:
		c.onWatch(message)
	case *messages.UnwatchMessage:
		c.onUnwatch(message)
	case *SchedulerMessage:
		c.onScheduler(message, behavior)
	default:
		c.executeBehaviorWithRecovery(behavior)
	}
}

func (c *Context) executeBehaviorWithRecovery(behavior vivid.Behavior) {
	defer func() {
		if r := recover(); r != nil {
			c.Logger().Error("unexpected error", log.Any("error", r), log.String("error_type", fmt.Sprintf("%T", r)), log.String("stack", string(debug.Stack())))
			fmt.Println(string(debug.Stack()))
			c.Failed(r)
		}
	}()
	behavior(c)
}

func (c *Context) onScheduler(message *SchedulerMessage, behavior vivid.Behavior) {
	// 检查是否过期
	if !c.scheduler.Exists(message.Reference) {
		c.Logger().Warn("scheduler message expired", log.String("reference", message.Reference))
		return
	}

	// 消息替换
	c.envelop = newReplacedEnvelop(c.envelop, message.Message)

	// 执行调度
	if message.Once {
		if err := c.scheduler.Cancel(message.Reference); err != nil {
			c.Logger().Error("failed to cancel scheduler", log.String("reference", message.Reference), log.Any("error", err))
		}
	}
	c.executeBehaviorWithRecovery(behavior)
}

func (c *Context) onCommand(message *messages.NoneArgsCommandMessage) {
	switch message.Command {
	case messages.CommandPauseMailbox:
		c.mailbox.Pause()
		// 通知事件流
		c.EventStream().Publish(c, ves.ActorMailboxPausedEvent{
			ActorRef: c.ref,
			Type:     reflect.TypeOf(c.actor),
		})
	case messages.CommandResumeMailbox:
		c.mailbox.Resume()
		// 通知事件流
		c.EventStream().Publish(c, ves.ActorMailboxResumedEvent{
			ActorRef: c.ref,
			Type:     reflect.TypeOf(c.actor),
		})
	}
}

func (c *Context) onPing(message *messages.PingMessage) {
	pongMessage := &messages.PongMessage{
		Ping:        message,
		RespondTime: time.Now(),
	}
	c.Reply(pongMessage)
}

func (c *Context) onWatch(_ *messages.WatchMessage) {
	sender := c.envelop.Sender()
	// 父节点不需要显式监听子节点，因为父节点会自动监听子节点
	if sender.Equals(c.parent) {
		c.Logger().Debug("parent does not need to watch child explicitly; this is handled by default", log.String("ref", c.ref.GetPath()), log.String("address", sender.GetAddress()), log.String("path", sender.GetPath()))
		return
	}

	// 如果已死，直接告知死亡
	if atomic.LoadInt32(&c.state) == killed {
		c.Logger().Debug("watcher already killed", log.String("ref", c.ref.GetPath()), log.String("address", sender.GetAddress()), log.String("path", sender.GetPath()))
		c.tell(false, sender, &vivid.OnKilled{Ref: c.ref})
		return
	}

	// 检查是否已经监听
	full := fmt.Sprintf("%s@%s", sender.GetAddress(), sender.GetPath())
	if _, exists := c.watchers[full]; exists {
		c.Logger().Debug("watcher already exists", log.String("ref", c.ref.GetPath()), log.String("address", sender.GetAddress()), log.String("path", sender.GetPath()))
		return
	}

	if c.watchers == nil {
		c.watchers = make(map[string]vivid.ActorRef)
	}

	c.watchers[full] = sender
	c.Logger().Debug("watcher added", log.String("ref", c.ref.GetPath()), log.String("address", sender.GetAddress()), log.String("path", sender.GetPath()))
	// 通知事件流
	c.EventStream().Publish(c, ves.ActorWatchedEvent{
		ActorRef: c.ref,
		Watcher:  sender,
	})
}

func (c *Context) onUnwatch(_ *messages.UnwatchMessage) {
	sender := c.envelop.Sender()
	full := fmt.Sprintf("%s@%s", sender.GetAddress(), sender.GetPath())
	if _, exists := c.watchers[full]; !exists {
		c.Logger().Debug("watcher not found", log.String("ref", c.ref.GetPath()), log.String("address", sender.GetAddress()), log.String("path", sender.GetPath()))
		return
	}
	delete(c.watchers, full)
	c.Logger().Debug("watcher removed", log.String("ref", c.ref.GetPath()), log.String("address", sender.GetAddress()), log.String("path", sender.GetPath()))
	// 通知事件流
	c.EventStream().Publish(c, ves.ActorUnwatchedEvent{
		ActorRef: c.ref,
		Watcher:  sender,
	})
}

func (c *Context) onRestart(message *RestartMessage, behavior vivid.Behavior) {
	// restart 只允许从 running 进入，避免覆盖并发的 kill/stop 流程
	if !atomic.CompareAndSwapInt32(&c.state, running, killing) {
		return
	}

	// 标记正在重启
	c.restarting = message
	c.Logger().Debug("receive restart", log.String("path", c.ref.GetPath()), log.String("reason", message.Reason), log.Any("fault", message.Fault), log.String("stack", string(message.Stack)))

	// 通知事件流
	c.EventStream().Publish(c, ves.ActorRestartingEvent{
		ActorRef: c.ref,
		Type:     reflect.TypeOf(c.actor),
		Reason:   message.Reason,
		Fault:    message.Fault,
	})

	// 执行重启前的预处理
	// 该阶段主要是用户的自定义清理逻辑，发生异常应当记录错误并继续重启
	if preRestartActor, ok := c.actor.(vivid.PreRestartActor); ok {
		message.recoverExec(c.Logger(), "pre restart", false, func() error {
			return preRestartActor.OnPreRestart(c)
		})
	}

	// 结束自身，由于重启已经进行了优雅处理筛选，应当立即执行
	// 覆盖当前消息，以便在 onKill 中使用
	killMessage := &vivid.OnKill{
		Killer: c.ref,
		Poison: message.Poison,
		Reason: message.Reason,
	}
	c.envelop = mailbox.NewEnvelop(true, c.Sender(), c.ref, killMessage)
	c.doKill(killMessage, behavior)
}

func (c *Context) onKill(message *vivid.OnKill, behavior vivid.Behavior) {
	if !atomic.CompareAndSwapInt32(&c.state, running, killing) {
		return
	}
	c.doKill(message, behavior)
}

func (c *Context) doKill(message *vivid.OnKill, behavior vivid.Behavior) {
	c.Logger().Debug("receive kill", log.String("path", c.ref.path), log.Bool("restarting", c.restarting != nil))
	// 等待所有子 Actor 结束，假设是重启，子 Actor 不应该跟随重启，应该由父节点决定是否重启
	for _, child := range c.children {
		c.Logger().Debug("notify child kill", log.String("path", child.GetPath()))
		c.Kill(child, message.Poison, message.Reason)
	}

	// 宣告自己进入死亡中
	if c.restarting != nil {
		// 失败意味着资源可能无法正确释放，但不应阻止新实例的创建。
		// 可能存在资源泄漏，应当记录警告
		logger := c.Logger()
		if !c.restarting.recoverExec(logger, "on kill", false, func() error {
			behavior(c)
			return nil
		}) {
			logger.Warn("restart kill failed; resources may not have been properly released", log.String("path", c.ref.GetPath()), log.String("reason", c.restarting.Reason), log.Any("fault", c.restarting.Fault), log.String("stack", string(c.restarting.Stack)))
		}
	} else {
		behavior(c)
	}

	// 尝试确认死亡
	c.onKilled(&vivid.OnKilled{Ref: c.ref}, behavior)
}

func (c *Context) onKilled(message *vivid.OnKilled, behavior vivid.Behavior) {
	handler := newKilledHandler(c, message, behavior)

	if err := chain.New().
		Append(chain.ChainFN(handler.handleChildDeath)).
		Append(chain.ChainFN(handler.checkAndMarkKilled)).
		Append(chain.ChainFN(handler.prepareSelfKilledMessage)).
		Append(chain.ChainFN(handler.executeBehavior)).
		Append(chain.ChainFN(handler.cleanupIfNotRestarting)).
		Append(chain.ChainFN(handler.cleanupScheduler)).
		Append(chain.ChainFN(handler.handleRestart)).
		Run(); err != nil {
		c.Logger().Error("onKilled failed", log.Any("error", err))
	}
}

func (c *Context) Message() vivid.Message {
	return c.envelop.Message()
}

func (c *Context) Sender() vivid.ActorRef {
	if agent := c.envelop.Agent(); agent != nil {
		return agent
	}
	return c.envelop.Sender()
}

func generateBehaviorOptions(options ...vivid.BehaviorOption) *vivid.BehaviorOptions {
	var opts = &vivid.BehaviorOptions{
		DiscardOld: true,
	}
	for _, option := range options {
		option(opts)
	}
	return opts
}

func (c *Context) Become(behavior vivid.Behavior, options ...vivid.BehaviorOption) {
	var opts = generateBehaviorOptions(options...)

	if opts.DiscardOld {
		c.behaviorStack.Clear()
	}
	c.behaviorStack.Push(behavior)
}

func (c *Context) UnBecome(options ...vivid.BehaviorOption) {
	var opts = generateBehaviorOptions(options...)

	if opts.DiscardOld {
		c.behaviorStack.Clear()
	} else {
		c.behaviorStack.Pop()
	}

	if c.behaviorStack.Len() == 0 {
		c.behaviorStack.Push(c.actor.OnReceive)
	}
}

func (c *Context) Kill(ref vivid.ActorRef, poison bool, reason ...string) {
	// poison 为 true 时，作为用户消息处理，否则作为系统消息处理
	c.tell(!poison, ref, &vivid.OnKill{
		Killer: c.ref,
		Poison: poison,
		Reason: strings.Join(reason, ", "),
	})
}

func (c *Context) Children() vivid.ActorRefs {
	children := make(vivid.ActorRefs, 0, len(c.children))
	for _, child := range c.children {
		children = append(children, child)
	}
	return children
}

// Failed 对于 vivid.ActorContext 的实现，该函数是并发安全的
func (c *Context) Failed(fault vivid.Message) {
	// 挂起当前 Actor 的消息处理并且向父级 Actor 发送监督上下文以触发父级 Actor 的监督策略
	c.mailbox.Pause()
	supervisionContext := newSupervisionContext(c.ref, fault)
	c.tell(true, c.parent, supervisionContext)
	// 通知事件流
	c.EventStream().Publish(c, ves.ActorFailedEvent{
		ActorRef: c.ref,
		Type:     reflect.TypeOf(c.actor),
		Fault:    fault,
	})
	// 通知事件流：邮箱暂停
	c.EventStream().Publish(c, ves.ActorMailboxPausedEvent{
		ActorRef: c.ref,
		Type:     reflect.TypeOf(c.actor),
	})
}

func (c *Context) onSupervise(supervisionContext *supervisionContext) {
	supervise(c, supervisionContext)
	var (
		targets  vivid.ActorRefs
		decision vivid.SupervisionDecision
		reason   string
	)

	// 获取影响的目标和决策
	targets, decision, reason = c.options.SupervisionStrategy.Supervise(supervisionContext)

	// 暂停所有目标的邮箱消息处理
	mailboxPauseMessage := messages.CommandPauseMailbox.Build()
	for _, target := range targets {
		c.tell(true, target, mailboxPauseMessage)
	}
	supervisionContext.applyDecision(c, targets, decision, reason)
}

// 目前该消息暂无任何字段，将其固化避免额外的内存分配
var watchMessage = new(messages.WatchMessage)

func (c *Context) Watch(ref vivid.ActorRef) {
	c.tell(true, ref, watchMessage)
}

// 目前该消息暂无任何字段，将其固化避免额外的内存分配
var unwatchMessage = new(messages.UnwatchMessage)

func (c *Context) Unwatch(ref vivid.ActorRef) {
	c.tell(true, ref, unwatchMessage)
}
