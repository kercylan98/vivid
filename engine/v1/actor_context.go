package vivid

import (
	"fmt"
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/engine/v1/future"
	"github.com/kercylan98/vivid/engine/v1/internal/builtinfuture"
	"github.com/kercylan98/vivid/engine/v1/internal/builtinmailbox"
	"github.com/kercylan98/vivid/engine/v1/internal/processor"
	"github.com/kercylan98/vivid/engine/v1/mailbox"
	"github.com/kercylan98/vivid/src/queues"
	"sync/atomic"
	"time"
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

type ActorContext interface {
	Logger() log.Logger

	Ref() ActorRef

	ActorOf(provider ActorProviderFN) ActorGenerator

	ActorOfP(provider ActorProvider) ActorGenerator

	SpawnOf(provider ActorProviderFN) ActorRef

	SpawnOfP(provider ActorProvider) ActorRef

	Tell(target ActorRef, message Message)

	Probe(target ActorRef, message Message)

	// Kill 立即终止指定的 Actor。
	// 发送系统消息，立即开始终止流程。
	Kill(target ActorRef, reason ...string)

	// PoisonKill 优雅终止指定的 Actor。
	// 发送用户消息，在处理完当前队列后开始终止流程。
	PoisonKill(target ActorRef, reason ...string)

	// Ask 向指定的 Actor 发送一条异步消息。
	Ask(target ActorRef, message Message, timeout ...time.Duration) future.Future

	// Reply 向当前 Actor 的发送者发送一条消息。
	Reply(message Message)

	// Sender 获取当前正在处理的消息的发送者。
	Sender() ActorRef

	// Message 获取当前正在处理的消息。
	Message() Message
}

func newActorContext(system *actorSystem, ref, parent ActorRef, provider ActorProvider, config *ActorConfiguration) *actorContext {
	if parent == nil {
		system.shutdownWG.Add(1)
	}

	ctx := &actorContext{
		system:   system,
		parent:   parent,
		provider: provider,
		config:   *config,
		ref:      ref,
	}

	if ctx.config.Logger == nil {
		ctx.config.Logger = log.GetDefault()
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

	ctx.actor = provider.Provide()
	return ctx
}

type actorContext struct {
	system     *actorSystem        // ActorContext 所属的 ActorSystem。
	config     ActorConfiguration  // ActorContext 的配置。
	provider   ActorProvider       // ActorContext 的 ActorProvider。
	parent     ActorRef            // ActorContext 的父 Actor 引用，顶级 Actor 为 nil。
	ref        ActorRef            // ActorContext 自身的引用。
	mailbox    mailbox.Mailbox     // ActorContext 的邮箱。
	childGuid  int64               // ActorContext 的子 Actor GUID，用于生成子 Actor 引用。
	children   map[string]ActorRef // ActorContext 的子 Actor 引用映射。
	actor      Actor               // Actor 实例。
	sender     ActorRef            // 当前正在处理的消息的发送者。
	message    Message             // 当前正在处理的消息。
	state      uint32              // Actor 状态。
	killedInfo *OnKilled           // 记录终止 Actor 的信息
}

func (ctx *actorContext) OnSystemMessage(message any) {
	ctx.sender, ctx.message = unwrapMessage(message)
	switch msg := ctx.message.(type) {
	case *OnLaunch:
		ctx.onReceiveWithRecover()
	case *OnKill:
		ctx.onKill(msg)
	case *OnKilled:
		ctx.onKilled(msg)
	}
}

func (ctx *actorContext) OnUserMessage(message any) {
	ctx.sender, ctx.message = unwrapMessage(message)

	switch msg := ctx.message.(type) {
	case *OnKill:
		ctx.onKill(msg)
	default:
		ctx.onReceiveWithRecover()
	}
}

func (ctx *actorContext) HandleUserMessage(sender processor.UnitIdentifier, message any) {
	// 当 Actor 处于终止状态时，不再接收用户消息，处于终止中时还应该继续接收，但是邮箱会屏蔽用户消息的处理
	if atomic.LoadUint32(&ctx.state) == actorStateStopped {
		ctx.Logger().Debug("Rejecting user message, actor not running",
			log.String("actor", ctx.ref.GetPath()),
			log.Any("state", ctx.state))
		return
	}
	ctx.mailbox.PushUserMessage(message)
}

func (ctx *actorContext) HandleSystemMessage(sender processor.UnitIdentifier, message any) {
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
	unit.HandleUserMessage(ctx.ref, wrapMessage(ctx.ref, message))
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

	ref := ctx.system.Ref().Branch(fmt.Sprintf("future-%d", ctx.childGuid))
	f := builtinfuture.New(ctx.system.registry, ref, t)
	unit, err := ctx.system.registry.GetUnit(target)
	if err != nil {
		ctx.Logger().Error("Ask", log.Err(err))
		return f
	}
	unit.HandleUserMessage(ref, wrapMessage(ref, message))
	return f
}

func (ctx *actorContext) Reply(message Message) {
	ctx.Tell(ctx.sender, message)
}

func (ctx *actorContext) systemTell(target ActorRef, message Message) {
	unit, err := ctx.system.registry.GetUnit(target)
	if err != nil {
		ctx.Logger().Error("systemTell", log.Err(err))
		return
	}
	unit.HandleSystemMessage(ctx.ref, message)
}

func (ctx *actorContext) systemProbe(target ActorRef, message Message) {
	unit, err := ctx.system.registry.GetUnit(target)
	if err != nil {
		ctx.Logger().Error("systemProbe", log.Err(err))
		return
	}
	unit.HandleSystemMessage(ctx.ref, wrapMessage(ctx.ref, message))
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
	ctx.onReceiveWithRecover()

	// 终止所有子 Actor
	for _, ref := range ctx.children {
		if onKill.IsPoison() {
			ctx.PoisonKill(ref, onKill.Reason()...)
		} else {
			ctx.Kill(ref, onKill.Reason()...)
		}
	}

	// 终止所有子 Actor
	ctx.tryConvertStateToStopping()
}

func (ctx *actorContext) onKilled(msg *OnKilled) {
	// 解绑已终止的子 Actor
	ctx.unbindChild(msg.ref)
	ctx.onReceiveWithRecover()
	ctx.tryConvertStateToStopping()
}

// tryConvertStateToStopping 尝试将状态转换为停止状态，需要注意 onKilled 可能是来自子 Actor 的终止消息
func (ctx *actorContext) tryConvertStateToStopping() {
	// 如果子 Actor 已全部终止，完成自身终止
	if len(ctx.children) > 0 {
		return
	}

	// 状态变更
	if !atomic.CompareAndSwapUint32(&ctx.state, actorStateStopping, actorStateStopped) {
		return
	}

	// 取消处理单元注册
	ctx.system.registry.UnregisterUnit(ctx.ref, ctx.ref)

	// 通知父 Actor，如果不使用系统消息，会因为邮箱已经暂停而导致无法通知终止中的父 Actor
	if ctx.parent != nil {
		ctx.systemTell(ctx.parent, ctx.killedInfo)
	} else {
		ctx.system.shutdownWG.Done()
	}
}

func (ctx *actorContext) Ref() ActorRef {
	return ctx.ref
}

func (ctx *actorContext) Sender() ActorRef {
	return ctx.sender
}

func (ctx *actorContext) Message() Message {
	return ctx.message
}

func (ctx *actorContext) onReceiveWithRecover() {
	defer func() {
		if r := recover(); r != nil {
			ctx.Logger().Error("ActorContext OnReceive panic", log.Any("panic", r))
		}
	}()
	ctx.actor.Receive(ctx)
}
