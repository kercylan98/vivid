package vivid

import (
	"fmt"
	"github.com/kercylan98/vivid/pkg/vivid/future"
	"github.com/kercylan98/vivid/pkg/vivid/internal/builtinfuture"
	"github.com/kercylan98/vivid/pkg/vivid/internal/builtinmailbox"
	"github.com/kercylan98/vivid/pkg/vivid/internal/processor"
	"github.com/kercylan98/vivid/pkg/vivid/mailbox"
	"golang.org/x/exp/maps"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/pkg/queues"
)

var _ ActorContext = (*actorContext)(nil)
var _ processor.Unit = (*actorContext)(nil)

const (
	// actorStateRunning 表示 Actor 正在运行。
	actorStateRunning = iota
	// actorStateStopping 表示 Actor 正在终止。
	actorStateStopping
	// actorStateStopped 表示 Actor 已终止。
	actorStateStopped
)

// SystemContext 定义了 Actor 系统中通用上下文，它们是 Actor 系统和 Actor 共有的上下文。
type SystemContext interface {
	// Logger 返回当前 Actor 关联的日志记录器。
	//
	// 每个 Actor 都有自己的日志记录器，通常包含 Actor 的路径信息。
	// 返回的日志记录器可以用于记录 Actor 的运行状态和调试信息。
	Logger() log.Logger

	// Ref 返回当前 Actor 的引用。
	//
	// Actor 引用是 Actor 的唯一标识，可以用于消息发送和 Actor 定位。
	// 返回当前 Actor 的 ActorRef 实例。
	Ref() ActorRef

	// Parent 返回当前 Actor 的父 Actor 引用。
	//
	// 在 Actor 层次结构中，每个 Actor（除了根 Actor）都有一个父 Actor。
	// 如果当前 Actor 是顶级 Actor，则返回 nil。
	Parent() ActorRef

	// ActorOf 创建一个 Actor 生成器，使用函数式 Provider。
	//
	// 生成器模式允许灵活配置 Actor 的创建参数。
	// 参数 provider 是创建 Actor 实例的函数式提供者。
	// 返回一个 ActorGenerator，可以进一步配置并创建 Actor。
	ActorOf(provider ActorProviderFN) ActorGenerator

	// ActorOfP 创建一个 Actor 生成器，使用接口式 Provider。
	//
	// 与 ActorOf 类似，但使用接口式的 Provider。
	// 参数 provider 是创建 Actor 实例的接口式提供者。
	// 返回一个 ActorGenerator，可以进一步配置并创建 Actor。
	ActorOfP(provider ActorProvider) ActorGenerator

	// SpawnOf 直接创建并启动一个子 Actor，使用函数式 Provider。
	//
	// 这是创建 Actor 的快捷方式，使用默认配置。
	// 参数 provider 是创建 Actor 实例的函数式提供者。
	// 返回新创建的 Actor 引用。
	SpawnOf(provider ActorProviderFN) ActorRef

	// SpawnOfP 直接创建并启动一个子 Actor，使用接口式 Provider。
	//
	// 与 SpawnOf 类似，但使用接口式的 Provider。
	// 参数 provider 是创建 Actor 实例的接口式提供者。
	// 返回新创建的 Actor 引用。
	SpawnOfP(provider ActorProvider) ActorRef

	// Tell 向目标 Actor 发送一条单向消息（无需响应）。
	//
	// Tell 是最基本的消息发送方式，采用"发送并忘记"的模式。
	// 发送者不会等待响应，也不会知道消息是否被成功处理。
	//
	// 参数：
	//   - target: 目标 Actor 的引用
	//   - message: 要发送的消息，可以是任意类型
	Tell(target ActorRef, message Message)

	// Probe 向目标 Actor 发送一条探测消息（带发送者信息）。
	//
	// 与 Tell 类似，但会在消息中包含发送者信息。
	// 接收者可以通过 Sender() 方法获取发送者引用并回复消息。
	//
	// 参数：
	//   - target: 目标 Actor 的引用
	//   - message: 要发送的消息，可以是任意类型
	Probe(target ActorRef, message Message)

	// Kill 立即终止指定的 Actor。
	//
	// Kill 是强制终止方式，Actor 会立即停止，不等待当前消息处理完成。
	// 这种方式可能导致数据丢失，应谨慎使用。
	//
	// 参数：
	//   - target: 要终止的 Actor 引用
	//   - reason: 终止原因，用于日志记录和调试
	Kill(target ActorRef, reason ...string)

	// PoisonKill 优雅终止指定的 Actor。
	//
	// PoisonKill 是优雅终止方式，Actor 会等待当前消息处理完成后停止。
	// 这是推荐的 Actor 终止方式，可以确保数据一致性。
	//
	// 参数：
	//   - target: 要终止的 Actor 引用
	//   - reason: 终止原因，用于日志记录和调试
	PoisonKill(target ActorRef, reason ...string)

	// Ask 向指定的 Actor 发送一条异步消息并返回一个 future.Future 对象。
	//
	// Ask 模式用于需要响应的消息发送，返回一个 Future 对象来获取响应。
	// 发送者可以选择立即等待响应或稍后获取结果。
	//
	// 参数：
	//   - target: 目标 Actor 的引用
	//   - message: 要发送的消息，可以是任意类型
	//   - timeout: 可选的超时时间，如果不指定则使用系统默认值
	//
	// 返回一个 Future 对象，可以用来获取响应结果。
	Ask(target ActorRef, message Message, timeout ...time.Duration) future.Future
}

// ActorContext 定义了 Actor 运行时上下文的接口。
//
// ActorContext 是 Actor 与系统交互的主要接口，提供了 Actor 运行所需的所有功能。
// 它包含了消息传递、Actor 管理、日志记录等核心能力。
//
// 主要功能分类：
//   - 消息传递：Tell、Probe、Ask、Reply
//   - Actor 管理：ActorOf、SpawnOf、Kill、PoisonKill
//   - 上下文信息：Ref、Parent、Sender、Message
//   - 系统服务：Logger
type ActorContext interface {
	SystemContext

	// Reply 向当前消息的发送者回复一条消息。
	//
	// 只有在处理通过 Probe 或 Ask 发送的消息时才能使用此方法。
	// 如果当前消息没有发送者信息，此方法不会产生任何效果。
	//
	// 参数 message 是要回复的消息，可以是任意类型。
	Reply(message Message)

	// Sender 获取当前正在处理的消息的发送者引用。
	//
	// 只有通过 Probe 或 Ask 发送的消息才会包含发送者信息。
	// 如果当前消息是通过 Tell 发送的，则返回 nil。
	Sender() ActorRef

	// Message 获取当前正在处理的消息内容。
	//
	// 返回当前 Actor 正在处理的消息对象。
	// 这通常在 Actor 的 Receive 方法中使用。
	Message() Message

	// AsPersistent 将当前上下文转换为持久化上下文。
	//
	// 如果当前 Actor 支持持久化，返回持久化上下文；否则返回 nil。
	// 用户可以在 Receive 方法中调用此方法来获取持久化功能。
	AsPersistent() PersistenceContext

	// Watch 监视指定 Actor 生命周期的停止。
	//
	// 当指定 Actor 停止时，当前 Actor 会收到 OnWatchEnd 消息。
	Watch(target ActorRef)

	// Unwatch 停止对指定的 Actor 生命周期的监视。
	Unwatch(target ActorRef)

	// AttachTask 以当前上下文为基准附加一个任务，该任务将被放入邮箱的末端进行处理。
	// 通过该函数将允许基于当前作用域执行延迟任务，例如作为异步的回调任务来执行。
	//
	// 在 ActorContextTask 中获得的 Sender 以及 Message 均是当前上下文所能得到的结果。
	AttachTask(task ActorContextTask)

	// Future 创建一个异步任务，并将结果投递到目标 Actor。
	Future(ref ActorRef, task FutureTask, failureHandlers ...FutureTaskFailureHandler)

	// ChildNum 获取子 Actor 数量，该数量不会包含孙级别及以下的 Actor。
	ChildNum() int

	// Children 获取子 Actor 引用
	Children() []ActorRef
}

func newActorContext(system *actorSystem, ref, parent ActorRef, provider ActorProvider, config *ActorConfiguration) *actorContext {
	if parent == nil {
		system.shutdownWG.Add(1)
	}

	if config.RouterConfig != nil {
		provider = newRouterActorProvider(config.RouterConfig, provider, config)
		config.RouterConfig = nil // 子 Actor 不以路由器方式运行
	}

	ctx := &actorContext{
		system:   system,
		parent:   parent,
		provider: provider,
		config:   *config,
		ref:      ref,
	}

	if ctx.config.Logger == nil {
		ctx.config.Logger = system.Logger().WithGroup(fmt.Sprintf("[%s]", ctx.ref.GetPath()))
	}

	// 初始化邮箱
	if ctx.config.MailboxProvider != nil {
		ctx.mailbox = ctx.config.MailboxProvider.Provide()
	} else {
		ctx.mailbox = builtinmailbox.NewMailbox(
			queues.NewRingBuffer(32),
			queues.NewRingBuffer(32),
			builtinmailbox.NewDispatcher(ctx),
		)
	}

	// 创建 Actor 实例
	actor := provider.Provide()

	// 检查是否为持久化 Actor 并且配置了持久化
	if persistentActor, ok := actor.(PersistentActor); ok && ctx.config.PersistenceConfig != nil {
		// 创建持久化包装器
		ctx.actor = newPersistentActorWrapper(persistentActor, ctx.config.PersistenceConfig.Store, ctx.config.PersistenceConfig)
	} else {
		ctx.actor = actor
	}

	return ctx
}

type actorContext struct {
	system         *actorSystem        // ActorContext 所属的 ActorSystem。
	config         ActorConfiguration  // ActorContext 的配置。
	provider       ActorProvider       // ActorContext 的 ActorProvider。
	parent         ActorRef            // ActorContext 的父 Actor 引用，顶级 Actor 为 nil。
	ref            ActorRef            // ActorContext 自身的引用。
	mailbox        mailbox.Mailbox     // ActorContext 的邮箱。
	childGuid      atomic.Int64        // ActorContext 的子 Actor GUID，用于生成子 Actor 引用。
	children       map[string]ActorRef // ActorContext 的子 Actor 引用映射。
	actor          Actor               // Actor 实例。
	sender         ActorRef            // 当前正在处理的消息的发送者。
	message        Message             // 当前正在处理的消息。
	state          uint32              // Actor 状态。
	killedInfo     *OnKilled           // 记录终止 Actor 的信息
	fatal          *Fatal              // 当前致命错误信息
	restarting     bool                // Actor 是否正在重启中。
	persistenceCtx PersistenceContext  // 缓存的持久化上下文
	watches        map[string]ActorRef // 监视的 Actor 引用
}

func (ctx *actorContext) OnSystemMessage(message any) {
	startAt := time.Now()
	ctx.sender, ctx.message = processor.UnwrapMessage(message)
	ctx.system.hooks.trigger(actorHandleSystemMessageBeforeHookType, ctx.sender, ctx.ref, message)

	switch msg := ctx.message.(type) {
	case *OnLaunch:
		ctx.onLaunch(msg)
	case *OnKill:
		ctx.onKill(msg)
	case *OnKilled:
		ctx.onKilled(msg)
	case *Fatal:
		ctx.handleFatal(msg)
	case SupervisorDirective:
		ctx.executeSupervisorDirective(msg)
	case *OnPreRestart:
		ctx.onPreRestart()
	case *OnRestart:
		ctx.onRestart()
	case *onWatch:
		ctx.onWatch(msg)
	case *onUnwatch:
		ctx.onUnwatch(msg)
	}

	ctx.system.hooks.trigger(actorHandleSystemMessageAfterHookType, ctx.sender, ctx.ref, message, time.Since(startAt))
}

func (ctx *actorContext) OnUserMessage(message any) {
	var startAt *time.Time
	if ctx.system.hooks.hasHook(actorHandleUserMessageAfterHookType) {
		now := time.Now()
		startAt = &now
	}

	ctx.sender, ctx.message = processor.UnwrapMessage(message)

	ctx.system.hooks.trigger(actorHandleUserMessageBeforeHookType, ctx.sender, ctx.ref, message)

	switch msg := ctx.message.(type) {
	case *OnKill:
		ctx.onKill(msg)
	case *taskContext:
		msg.handle()
	default:
		ctx.onSafeReceive()
	}

	if startAt != nil {
		ctx.system.hooks.trigger(actorHandleUserMessageAfterHookType, ctx.sender, ctx.ref, message, time.Since(*startAt))
	}
}

func (ctx *actorContext) ChildNum() int {
	return len(ctx.children)
}

func (ctx *actorContext) Children() []ActorRef {
	return maps.Values(ctx.children)
}

func (ctx *actorContext) HandleUserMessage(_ processor.UnitIdentifier, message any) {
	// 当 Actor 处于终止状态时，不再接收用户消息，处于终止中时还应该继续接收，但是邮箱会屏蔽用户消息的处理
	if atomic.LoadUint32(&ctx.state) == actorStateStopped {
		ctx.Logger().Debug("Rejecting user message, actor not running",
			log.String("actor", ctx.ref.GetPath()),
			log.Any("state", ctx.state),
			log.String("message", fmt.Sprintf("%#v", message)))
		return
	}

	ctx.system.hooks.trigger(actorMailboxPushUserMessageBeforeHookType, ctx.ref, message)

	ctx.mailbox.PushUserMessage(message)
}

func (ctx *actorContext) HandleSystemMessage(_ processor.UnitIdentifier, message any) {
	ctx.system.hooks.trigger(actorMailboxPushSystemMessageBeforeHookType, ctx.ref, message)

	ctx.mailbox.PushSystemMessage(message)
}

func (ctx *actorContext) Tell(target ActorRef, message Message) {
	unit, err := ctx.system.registry.GetUnit(target)
	if err != nil {
		ctx.Logger().Error("Tell", log.Err(err))
		return
	}
	unit.HandleUserMessage(ctx.ref, message)
}

func (ctx *actorContext) Probe(target ActorRef, message Message) {
	unit, err := ctx.system.registry.GetUnit(target)
	if err != nil {
		ctx.Logger().Error("Probe", log.Err(err))
		return
	}
	unit.HandleUserMessage(ctx.ref, processor.WrapMessage(ctx.ref, message))
}

func (ctx *actorContext) Kill(target ActorRef, reason ...string) {
	ctx.systemTell(target, newOnKill(ctx.ref, false, reason))
}

func (ctx *actorContext) PoisonKill(target ActorRef, reason ...string) {
	ctx.Tell(target, newOnKill(ctx.ref, true, reason))
}

func (ctx *actorContext) Ask(target ActorRef, message Message, timeout ...time.Duration) future.Future {
	t := ctx.system.config.FutureDefaultTimeout
	if len(timeout) > 0 {
		t = timeout[0]
	}

	ref := ctx.Ref().Branch(fmt.Sprintf("future-%d", ctx.childGuid.Add(1)))
	f := builtinfuture.New(ctx.system.registry, ref, t)
	unit, err := ctx.system.registry.GetUnit(target)
	if err != nil {
		ctx.Logger().Error("Ask", log.Err(err))
		return f
	}
	unit.HandleUserMessage(ref, processor.WrapMessage(ref, message))
	return f
}

func (ctx *actorContext) Reply(message Message) {
	ctx.Tell(ctx.sender, message)
}

func (ctx *actorContext) systemTell(target ActorRef, message Message) {
	unit, err := ctx.system.registry.GetUnit(target)
	if err != nil {
		ctx.Logger().Error("systemTell", log.Any("target", target), log.Err(err))
		return
	}
	unit.HandleSystemMessage(ctx.ref, message)
}

func (ctx *actorContext) tell(sender, target ActorRef, message Message, system bool) {
	unit, err := ctx.system.registry.GetUnit(target)
	if err != nil {
		ctx.Logger().Error("tell", log.Any("target", target), log.Err(err))
		return
	}
	if system {
		unit.HandleSystemMessage(sender, message)
	} else {
		unit.HandleUserMessage(sender, message)
	}
}

func (ctx *actorContext) systemProbe(target ActorRef, message Message) {
	unit, err := ctx.system.registry.GetUnit(target)
	if err != nil {
		ctx.Logger().Error("systemProbe", log.Err(err))
		return
	}
	unit.HandleSystemMessage(ctx.ref, processor.WrapMessage(ctx.ref, message))
}

func (ctx *actorContext) probe(sender, target ActorRef, message Message, system bool) {
	unit, err := ctx.system.registry.GetUnit(target)
	if err != nil {
		ctx.Logger().Error("probe", log.Err(err))
		return
	}
	message = processor.WrapMessage(sender, message)
	if system {
		unit.HandleSystemMessage(sender, message)
	} else {
		unit.HandleUserMessage(sender, message)
	}
}

func (ctx *actorContext) Logger() log.Logger {
	return ctx.config.Logger
}

func (ctx *actorContext) ActorOf(provider ActorProviderFN) ActorGenerator {
	return newActorGenerator(ctx, provider)
}

func (ctx *actorContext) ActorOfP(provider ActorProvider) ActorGenerator {
	return newActorGenerator(ctx, provider)
}

func (ctx *actorContext) SpawnOf(provider ActorProviderFN) ActorRef {
	return ctx.ActorOf(provider).Spawn()
}

func (ctx *actorContext) SpawnOfP(provider ActorProvider) ActorRef {
	return ctx.ActorOfP(provider).Spawn()
}

func (ctx *actorContext) bindChild(ref ActorRef) {
	if ctx == nil {
		return
	}
	if ctx.children == nil {
		ctx.children = make(map[string]ActorRef)
	}
	ctx.children[ref.GetPath()] = ref
}

func (ctx *actorContext) unbindChild(ref ActorRef) {
	if ctx == nil {
		return
	}
	delete(ctx.children, ref.GetPath())
	if len(ctx.children) == 0 {
		ctx.children = nil
	}
}

// onKill 处理终止消息。
func (ctx *actorContext) onKill(onKill *OnKill) {
	if onKill.applied || !atomic.CompareAndSwapUint32(&ctx.state, actorStateRunning, actorStateStopping) {
		return
	}
	onKill.applied = true
	ctx.killedInfo = newOnKilled(onKill.operator, ctx.ref, onKill.IsPoison(), onKill.Reason())

	// 暂停邮箱继续处理用户消息
	// 此刻新的用户级消息继续被投递到邮箱中，但不会被执行
	ctx.mailbox.Suspend()

	// 等待用户处理关闭消息
	ctx.onSafeReceive()

	// 终止所有子 Actor
	for _, ref := range ctx.children {
		if onKill.IsPoison() {
			ctx.PoisonKill(ref, onKill.Reason()...)
		} else {
			ctx.Kill(ref, onKill.Reason()...)
		}
	}

	ctx.system.hooks.trigger(actorKillHookType, ctx, onKill)

	ctx.tryConvertStateToStopping()
}

func (ctx *actorContext) onKilled(msg *OnKilled) {
	// 解绑已终止的子 Actor
	ctx.unbindChild(msg.ref)
	ctx.onSafeReceive()
	ctx.tryConvertStateToStopping()
}

// tryConvertStateToStopping 尝试将状态转换为停止状态，需要注意 onKilled 可能是来自子 Actor 的终止消息
func (ctx *actorContext) tryConvertStateToStopping() {
	// 如果子 Actor 已全部终止，完成自身终止
	if len(ctx.children) > 0 {
		return
	}

	// 重启状态中
	if ctx.restarting {
		ctx.tryRestart()
	}

	// 状态变更
	if !atomic.CompareAndSwapUint32(&ctx.state, actorStateStopping, actorStateStopped) {
		return
	}

	// 取消处理单元注册
	ctx.system.registry.UnregisterUnit(ctx.ref, ctx.ref)

	// 触发钩子
	ctx.system.hooks.trigger(actorKilledHookType, ctx.killedInfo)

	// 通知监视者
	if len(ctx.watches) > 0 {
		watchEnd := &OnWatchEnd{
			ref:    ctx.ref,
			reason: ctx.killedInfo.Reason(),
		}
		for _, ref := range ctx.watches {
			ctx.Tell(ref, watchEnd)
		}
		ctx.watches = nil
	}

	// 通知父 Actor，如果不使用系统消息，会因为邮箱已经暂停而导致无法通知终止中的父 Actor
	if ctx.parent != nil {
		ctx.systemTell(ctx.parent, ctx.killedInfo)
	} else {
		ctx.system.shutdownWG.Done()
	}
}

func (ctx *actorContext) handleFatal(fatal *Fatal) {
	// 暂停邮箱继续处理用户消息
	if ctx.ref == fatal.Ref() {
		ctx.mailbox.Suspend()
		fatal.restartCount++
	}

	// 寻求监管策略
	var directive SupervisorDirective

	var escalate = ctx.config.SupervisionProvider == nil
	go func() {
		if !escalate {
			supervision := ctx.config.SupervisionProvider.Provide()
			directive = supervision.Strategy(fatal)
			escalate = directive == DirectiveEscalate
		}

		if escalate {
			ctx.systemTell(ctx.parent, fatal)
		} else {
			ctx.systemTell(fatal.Ref(), directive)
		}
	}()

}

func (ctx *actorContext) executeSupervisorDirective(directive SupervisorDirective) {
	switch directive {
	case DirectiveKill:
		ctx.Kill(ctx.ref, ctx.fatal.String())
	case DirectivePoisonKill:
		ctx.PoisonKill(ctx.ref, ctx.fatal.String())
		ctx.mailbox.Resume()
	case DirectiveResume:
		ctx.mailbox.Resume()
	case DirectiveRestart:
		ctx.systemTell(ctx.ref, &OnPreRestart{})
	case DirectiveEscalate:
		ctx.systemTell(ctx.parent, ctx.fatal)
	}
}

func (ctx *actorContext) onPreRestart() {
	if ctx.onSafeReceive() {
		return // 重启前发生异常，视为全新的错误
	}

	// 关闭所有子 Actor
	ctx.restarting = true
	var killReason = ctx.fatal.String()
	for _, ref := range ctx.children {
		ctx.PoisonKill(ref, killReason)
	}

	ctx.tryRestart()
}

func (ctx *actorContext) tryRestart() {
	if len(ctx.children) > 0 {
		return // 子 Actor 尚未全部终止
	}

	// 刷新 Actor 状态
	ctx.actor = ctx.provider.Provide()

	// 投递 OnRestart 消息
	ctx.restarting = false
	ctx.systemTell(ctx.ref, &OnRestart{})
}

func (ctx *actorContext) onRestart() {
	if ctx.onSafeReceive() {
		return // 重启前发生异常，视为全新的错误
	}

	// 处理初始化消息，不经过邮箱，避免中间出现其他消息导致初始化消息被丢弃
	ctx.OnSystemMessage(onLaunchInstance)
}

func (ctx *actorContext) Ref() ActorRef {
	return ctx.ref
}

func (ctx *actorContext) Parent() ActorRef {
	return ctx.parent
}

func (ctx *actorContext) Sender() ActorRef {
	return ctx.sender
}

func (ctx *actorContext) Message() Message {
	return ctx.message
}

func (ctx *actorContext) AsPersistent() PersistenceContext {
	// 如果已经缓存了持久化上下文，直接返回
	if ctx.persistenceCtx != nil {
		return ctx.persistenceCtx
	}

	// 检查当前 Actor 是否为持久化包装器
	if wrapper, ok := ctx.actor.(*persistentActorWrapper); ok {
		ctx.persistenceCtx = &persistenceContextImpl{
			actorContext: ctx,
			wrapper:      wrapper,
		}
		return ctx.persistenceCtx
	}
	return nil
}

func (ctx *actorContext) withFatalRecover(handler func()) (recovered bool) {
	defer func() {
		if r := recover(); r != nil {
			ctx.Logger().Error("panic", log.Any("reason", r))
			switch ctx.message.(type) {
			// 当发生此类消息如若作为致命错误会导致可能被监管策略反复重启从而引发不可预期的错误
			case *OnKill:
				return
			default:
				recovered = true
				ctx.fatal = newFatal(ctx, ctx.ref, ctx.message, r, debug.Stack())
				ctx.handleFatal(ctx.fatal)
			}
		}
	}()
	handler()
	return
}

func (ctx *actorContext) onSafeReceive() (recovered bool) {
	return ctx.withFatalRecover(func() {
		ctx.actor.Receive(ctx)
	})
}

func (ctx *actorContext) onLaunch(msg *OnLaunch) {
	if !ctx.onSafeReceive() {
		// 致命状态恢复、邮箱恢复
		ctx.fatal = nil
		ctx.mailbox.Resume()
	}
}

func (ctx *actorContext) Watch(target ActorRef) {
	ctx.systemProbe(target, &onWatch{})
}

func (ctx *actorContext) Unwatch(target ActorRef) {
	ctx.systemProbe(target, &onUnwatch{})
}

func (ctx *actorContext) onWatch(_ *onWatch) {
	// 如果 Actor 已停止，应当立即响应
	if atomic.LoadUint32(&ctx.state) == actorStateStopped {
		ctx.Reply(&OnWatchEnd{
			ref:    ctx.ref,
			reason: []string{"actor stopped"},
		})
		return
	}

	if ctx.watches == nil {
		ctx.watches = make(map[string]ActorRef)
	}
	ctx.watches[ctx.Sender().String()] = ctx.Sender()
}

func (ctx *actorContext) onUnwatch(_ *onUnwatch) {
	if ctx.watches == nil {
		return
	}
	delete(ctx.watches, ctx.Sender().String())
}

func (ctx *actorContext) AttachTask(task ActorContextTask) {
	ctx.Tell(ctx.ref, newTaskContext(ctx, task))
}

func (ctx *actorContext) Future(ref ActorRef, task FutureTask, failureHandlers ...FutureTaskFailureHandler) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				if len(failureHandlers) == 0 {
					ctx.Logger().Error("future panic", log.Any("reason", r))
					return
				}
				for _, handler := range failureHandlers {
					handler.OnFailure(ctx, ref, r)
				}
			}
		}()
		ctx.Tell(ref, task.Execute())
	}()
}
