package vivid

import (
	"context"
	"fmt"
	"github.com/kercylan98/vivid/core/vivid/future"
	"github.com/kercylan98/vivid/core/vivid/internal/builtinfuture"
	builtinmailbox2 "github.com/kercylan98/vivid/core/vivid/internal/builtinmailbox"
	processor2 "github.com/kercylan98/vivid/core/vivid/internal/processor"
	"github.com/kercylan98/vivid/core/vivid/mailbox"
	"math"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/src/queues"
)

var _ ActorContext = (*actorContext)(nil)
var _ processor2.Unit = (*actorContext)(nil)

const (
	// actorStateRunning 表示 Actor 正在运行。
	actorStateRunning = iota
	// actorStateStopping 表示 Actor 正在终止。
	actorStateStopping
	// actorStateStopped 表示 Actor 已终止。
	actorStateStopped
)

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
		ctx.config.Logger = system.Logger().WithGroup(fmt.Sprintf("[%s]", ctx.ref.GetPath()))
	}

	// 初始化邮箱
	if ctx.config.MailboxProvider != nil {
		ctx.mailbox = ctx.config.MailboxProvider.Provide()
	} else {
		ctx.mailbox = builtinmailbox2.NewMailbox(
			queues.NewRingBuffer(32),
			queues.NewRingBuffer(32),
			builtinmailbox2.NewDispatcher(ctx),
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
	childGuid      int64               // ActorContext 的子 Actor GUID，用于生成子 Actor 引用。
	children       map[string]ActorRef // ActorContext 的子 Actor 引用映射。
	actor          Actor               // Actor 实例。
	sender         ActorRef            // 当前正在处理的消息的发送者。
	message        Message             // 当前正在处理的消息。
	state          uint32              // Actor 状态。
	killedInfo     *OnKilled           // 记录终止 Actor 的信息
	fatal          *Fatal              // 当前致命错误信息
	restarting     bool                // Actor 是否正在重启中。
	persistenceCtx PersistenceContext  // 缓存的持久化上下文
}

func (ctx *actorContext) OnSystemMessage(message any) {
	startAt := time.Now()
	ctx.sender, ctx.message = unwrapMessage(message)
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
	}

	ctx.system.hooks.trigger(actorHandleSystemMessageAfterHookType, ctx.sender, ctx.ref, message, time.Since(startAt))
}

func (ctx *actorContext) OnUserMessage(message any) {
	var startAt *time.Time
	if ctx.system.hooks.hasHook(actorHandleUserMessageAfterHookType) {
		startAt = new(time.Time)
		*startAt = time.Now()
	}

	ctx.sender, ctx.message = unwrapMessage(message)

	ctx.system.hooks.trigger(actorHandleUserMessageBeforeHookType, ctx.sender, ctx.ref, message)

	switch msg := ctx.message.(type) {
	case *OnKill:
		ctx.onKill(msg)
	default:
		ctx.onReceiveWithRecover()
	}

	if startAt != nil {
		ctx.system.hooks.trigger(actorHandleUserMessageAfterHookType, ctx.sender, ctx.ref, message, time.Since(*startAt))
	}
}

func (ctx *actorContext) HandleUserMessage(sender processor2.UnitIdentifier, message any) {
	// 当 Actor 处于终止状态时，不再接收用户消息，处于终止中时还应该继续接收，但是邮箱会屏蔽用户消息的处理
	if atomic.LoadUint32(&ctx.state) == actorStateStopped {
		ctx.Logger().Debug("Rejecting user message, actor not running",
			log.String("actor", ctx.ref.GetPath()),
			log.Any("state", ctx.state))
		return
	}

	ctx.system.hooks.trigger(actorMailboxPushUserMessageBeforeHookType, ctx.ref, message)

	ctx.mailbox.PushUserMessage(message)
}

func (ctx *actorContext) HandleSystemMessage(sender processor2.UnitIdentifier, message any) {
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
		ctx.Logger().Error("systemTell", log.Any("target", target), log.Err(err))
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

	ctx.system.hooks.trigger(actorKillHookType, ctx, onKill)

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
	if ctx.onReceiveWithRecover() {
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
	if ctx.onReceiveWithRecover() {
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

func (ctx *actorContext) onReceiveWithRecover() (recovered bool) {
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
	ctx.actor.Receive(ctx)
	return
}

func (ctx *actorContext) onLaunch(msg *OnLaunch) {
	if !ctx.onReceiveWithRecover() {
		// 致命状态恢复、邮箱恢复
		ctx.fatal = nil
		ctx.mailbox.Resume()
	}
}

// persistentActorWrapper 包装持久化 Actor，提供自动恢复功能
type persistentActorWrapper struct {
	actor          PersistentActor
	store          PersistenceStore
	config         *PersistenceConfiguration
	persistenceID  string
	sequenceNumber int64
	isRecovering   bool
	recovered      bool // 标记是否已完成恢复

	// 批量处理相关
	eventBatch   []Event
	batchTimer   *time.Timer
	lastSnapshot int64 // 上次快照时的序列号
}

// newPersistentActorWrapper 创建持久化 Actor 包装器
func newPersistentActorWrapper(actor PersistentActor, store PersistenceStore, config *PersistenceConfiguration) *persistentActorWrapper {
	return &persistentActorWrapper{
		actor:         actor,
		store:         store,
		config:        config,
		persistenceID: actor.PersistenceID(),
		eventBatch:    make([]Event, 0, config.EventBatchSize),
	}
}

// Receive 实现 Actor 接口，处理消息并自动管理持久化
func (w *persistentActorWrapper) Receive(ctx ActorContext) {
	// 处理生命周期消息
	switch ctx.Message().(type) {
	case *OnLaunch:
		// Actor 启动时进行恢复
		w.recover(ctx)
		// 调用原始 Actor 处理 OnLaunch
		w.actor.Receive(ctx)
		return

	case *OnRestart:
		// Actor 重启时重新恢复
		w.recovered = false // 重置恢复标志
		w.recover(ctx)
		// 调用原始 Actor 处理 OnRestart
		w.actor.Receive(ctx)
		return

	case *OnPreRestart:
		// 先让用户处理 OnPreRestart（可能会修改状态）
		w.actor.Receive(ctx)
		// 用户处理完成后，进行关键持久化
		w.handleCriticalPersistence(ctx)
		return

	case *OnKill:
		// 先让用户处理 OnKill（可能会进行最终状态更新）
		w.actor.Receive(ctx)
		// 用户处理完成后，进行关键持久化
		w.handleCriticalPersistence(ctx)
		return
	}

	// 处理普通消息前检查是否需要刷新批量事件
	if w.shouldFlushEvents() {
		if err := w.flushEvents(ctx); err != nil {
			ctx.Logger().Error("Failed to flush events", "error", err)
		}
	}

	// 调用原始 Actor 的 Receive 方法处理普通消息
	w.actor.Receive(ctx)

	// 消息处理后检查是否需要自动快照
	w.checkAutoSnapshot(ctx)
}

// handleCriticalPersistence 处理关键生命周期的持久化
func (w *persistentActorWrapper) handleCriticalPersistence(ctx ActorContext) {
	ctx.Logger().Info("Starting critical persistence", "persistenceID", w.persistenceID)

	// 刷新待处理事件
	if err := w.flushEventsWithRetry(ctx); err != nil {
		w.handlePersistenceFailure(ctx, err)
		return
	}

	// 创建快照
	if err := w.makeSnapshotWithRetry(ctx); err != nil {
		w.handlePersistenceFailure(ctx, err)
		return
	}

	ctx.Logger().Info("Critical persistence completed", "persistenceID", w.persistenceID)
}

// flushEventsWithRetry 带重试的事件刷新
func (w *persistentActorWrapper) flushEventsWithRetry(ctx ActorContext) error {
	if len(w.eventBatch) == 0 {
		return nil
	}

	return w.retryWithBackoff(ctx, func() error {
		return w.flushEvents(ctx)
	})
}

// makeSnapshotWithRetry 带重试的快照创建
func (w *persistentActorWrapper) makeSnapshotWithRetry(ctx ActorContext) error {
	return w.retryWithBackoff(ctx, func() error {
		return w.makeSnapshot(ctx)
	})
}

// retryWithBackoff 指数退避重试
func (w *persistentActorWrapper) retryWithBackoff(ctx ActorContext, fn func() error) error {
	maxRetries := 5                     // 最大重试次数
	baseDelay := 100 * time.Millisecond // 基础延迟

	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// 指数退避延迟
			delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt-1)))
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}

			ctx.Logger().Warn("Retrying critical persistence operation", "attempt", attempt, "delay", delay, "error", lastErr)

			time.Sleep(delay)
		}

		if err := fn(); err != nil {
			lastErr = err
			ctx.Logger().Error("Critical persistence operation failed", "attempt", attempt, "error", err)
			continue
		}

		// 成功
		if attempt > 0 {
			ctx.Logger().Info("Critical persistence operation succeeded after retry", "attempt", attempt)
		}
		return nil
	}

	return lastErr
}

// handlePersistenceFailure 处理持久化失败
func (w *persistentActorWrapper) handlePersistenceFailure(ctx ActorContext, err error) {
	ctx.Logger().Error("Critical persistence failed after all retries",
		"persistenceID", w.persistenceID,
		"error", err)

	// 交给用户存储层接口处理
	snapshot := newSnapshot(w.persistenceID, w.sequenceNumber, w.actor.Snapshot(), time.Now())
	w.store.OnPersistenceFailed(ctx, snapshot, w.eventBatch)
}

// recover 执行恢复流程 - 在 Actor 生命周期的正确时机执行
// 恢复失败时会触发致命错误，确保 Actor 不会在错误状态下运行
func (w *persistentActorWrapper) recover(ctx ActorContext) {
	w.isRecovering = true
	defer func() {
		w.isRecovering = false
	}()

	ctx.Logger().Info("Starting recovery", "persistenceID", w.persistenceID)

	// 加载最新快照
	snapshot, err := w.store.LoadSnapshot(context.Background(), w.persistenceID)
	if err != nil {
		ctx.Logger().Error("Failed to load snapshot", "error", err)
		// 恢复失败，触发致命错误
		panic(fmt.Errorf("persistence recovery failed: unable to load snapshot for %s: %w", w.persistenceID, err))
	}

	var fromSequenceNumber int64 = 0

	// 如果有快照，先恢复快照状态
	if snapshot != nil {
		if err := w.actor.RestoreFromSnapshot(snapshot.SnapshotData); err != nil {
			ctx.Logger().Error("Failed to restore from snapshot", "error", err)
			// 快照恢复失败，触发致命错误
			panic(fmt.Errorf("persistence recovery failed: unable to restore from snapshot for %s: %w", w.persistenceID, err))
		}
		w.sequenceNumber = snapshot.SequenceNumber
		w.lastSnapshot = snapshot.SequenceNumber
		fromSequenceNumber = snapshot.SequenceNumber + 1
		ctx.Logger().Info("Restored from snapshot",
			"persistenceID", w.persistenceID,
			"sequenceNumber", snapshot.SequenceNumber)
	}

	// 加载快照之后的所有事件
	events, err := w.store.LoadEvents(context.Background(), w.persistenceID, fromSequenceNumber)
	if err != nil {
		ctx.Logger().Error("Failed to load events", "error", err)
		// 事件加载失败，触发致命错误
		panic(fmt.Errorf("persistence recovery failed: unable to load events for %s: %w", w.persistenceID, err))
	}

	// 回放事件 - 设置消息并调用 Actor 处理
	originalMessage := ctx.Message()
	originalSender := ctx.Sender()

	for _, event := range events {
		// 临时设置事件数据为当前消息
		if actorCtx, ok := ctx.(*actorContext); ok {
			actorCtx.message = event.EventData
			actorCtx.sender = nil // 恢复期间没有发送者
		}

		// 调用 Actor 处理事件（如果处理失败会被上层的 panic 处理机制捕获）
		w.actor.Receive(ctx)
		w.sequenceNumber = event.SequenceNumber
	}

	// 恢复原始消息和发送者
	if actorCtx, ok := ctx.(*actorContext); ok {
		actorCtx.message = originalMessage
		actorCtx.sender = originalSender
	}

	// 只有恢复完全成功才标记为已恢复
	w.recovered = true
	ctx.Logger().Info("Recovery completed",
		"persistenceID", w.persistenceID,
		"sequenceNumber", w.sequenceNumber,
		"eventsReplayed", len(events))
}

// checkAutoSnapshot 检查是否需要自动创建快照
func (w *persistentActorWrapper) checkAutoSnapshot(ctx ActorContext) {
	if !w.config.EnableAutoSnapshot {
		return
	}

	// 检查是否达到快照间隔
	if w.sequenceNumber-w.lastSnapshot >= w.config.SnapshotInterval {
		if err := w.makeSnapshot(ctx); err != nil {
			ctx.Logger().Error("Auto snapshot failed", "error", err)
		}
	}
}

// makeSnapshot 创建快照
func (w *persistentActorWrapper) makeSnapshot(ctx ActorContext) error {
	snapshotData := w.actor.Snapshot()
	if snapshotData == nil {
		return nil // Actor 不需要创建快照
	}

	snapshot := newSnapshot(w.persistenceID, w.sequenceNumber, snapshotData, time.Now())

	if err := w.store.SaveSnapshot(context.Background(), snapshot); err != nil {
		return err
	}

	w.lastSnapshot = w.sequenceNumber
	ctx.Logger().Info("Snapshot created",
		"persistenceID", w.persistenceID,
		"sequenceNumber", w.sequenceNumber)
	return nil
}

// flushEvents 刷新批量事件到存储层
func (w *persistentActorWrapper) flushEvents(ctx ActorContext) error {
	if len(w.eventBatch) == 0 {
		return nil
	}

	// 批量保存事件
	for _, event := range w.eventBatch {
		if err := w.store.SaveEvent(context.Background(), event); err != nil {
			ctx.Logger().Error("Failed to save event", "error", err, "sequenceNumber", event.SequenceNumber)
			return err
		}
	}

	ctx.Logger().Debug("Events flushed",
		"persistenceID", w.persistenceID,
		"eventCount", len(w.eventBatch))

	// 清空批量缓存
	w.eventBatch = w.eventBatch[:0]

	// 重置定时器
	if w.batchTimer != nil {
		w.batchTimer.Stop()
		w.batchTimer = nil
	}

	return nil
}

// addEventToBatch 添加事件到批量缓存
func (w *persistentActorWrapper) addEventToBatch(ctx ActorContext, event Event) error {
	w.eventBatch = append(w.eventBatch, event)

	// 检查是否达到批量大小
	if len(w.eventBatch) >= w.config.EventBatchSize {
		return w.flushEvents(ctx)
	}

	// 如果还没有定时器，启动定时器
	// 注意：定时器到期时，我们不能直接调用 flushEvents，因为那时可能没有 ActorContext
	// 实际的刷新会在下次消息处理时检查并执行
	if w.batchTimer == nil {
		w.batchTimer = time.AfterFunc(w.config.EventFlushInterval, func() {
			// 定时器到期，标记需要刷新，实际刷新在下次消息处理时进行
			w.batchTimer = nil
		})
	}

	return nil
}

// shouldFlushEvents 检查是否应该刷新事件（在消息处理时调用）
func (w *persistentActorWrapper) shouldFlushEvents() bool {
	// 如果定时器已过期（batchTimer 为 nil）且有待处理事件，则需要刷新
	return w.batchTimer == nil && len(w.eventBatch) > 0
}

// persistenceContextImpl 实现 PersistenceContext 接口
type persistenceContextImpl struct {
	actorContext *actorContext
	wrapper      *persistentActorWrapper
}

func (p *persistenceContextImpl) IsRecovering() bool {
	return p.wrapper.isRecovering
}

func (p *persistenceContextImpl) LastSequenceNumber() int64 {
	return p.wrapper.sequenceNumber
}

func (p *persistenceContextImpl) PersistEvent(eventData Message, callback EventHandler) error {
	// 恢复期间不记录事件
	if p.wrapper.isRecovering {
		if callback != nil {
			callback.OnEventPersisted(Event{})
		}
		return nil
	}
	return p.persistEventInternal(eventData, callback, false)
}

func (p *persistenceContextImpl) PersistEventSync(eventData Message) error {
	// 恢复期间不记录事件
	if p.wrapper.isRecovering {
		return nil
	}
	return p.persistEventInternal(eventData, nil, true)
}

func (p *persistenceContextImpl) MakeSnapshot() error {
	// 恢复期间不保存快照
	if p.wrapper.isRecovering {
		return nil
	}

	// 先刷新所有待处理的事件
	if err := p.wrapper.flushEvents(p.actorContext); err != nil {
		return err
	}

	// 使用包装器的方法创建快照
	return p.wrapper.makeSnapshot(p.actorContext)
}

func (p *persistenceContextImpl) persistEventInternal(eventData Message, callback EventHandler, sync bool) error {
	// 生成新的序列号
	p.wrapper.sequenceNumber++

	event := Event{
		PersistenceID:  p.wrapper.persistenceID,
		SequenceNumber: p.wrapper.sequenceNumber,
		EventType:      "default",
		EventData:      eventData,
		Timestamp:      time.Now(),
	}

	if sync {
		// 同步模式：立即保存事件
		if err := p.wrapper.store.SaveEvent(context.Background(), event); err != nil {
			// 回滚序列号
			p.wrapper.sequenceNumber--
			return err
		}

		// 执行回调（用户在回调中更新状态）
		if callback != nil {
			callback.OnEventPersisted(event)
		}
	} else {
		// 异步模式：添加到批量缓存
		if err := p.wrapper.addEventToBatch(p.actorContext, event); err != nil {
			// 回滚序列号
			p.wrapper.sequenceNumber--
			return err
		}

		// 执行回调（用户在回调中更新状态）
		if callback != nil {
			callback.OnEventPersisted(event)
		}
	}

	return nil
}
