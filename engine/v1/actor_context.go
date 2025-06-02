package vivid

import (
    "github.com/kercylan98/go-log/log"
    "github.com/kercylan98/vivid/engine/v1/internal/builtinmailbox"
    "github.com/kercylan98/vivid/engine/v1/internal/processor"
    "github.com/kercylan98/vivid/engine/v1/mailbox"
    "github.com/kercylan98/vivid/src/queues"
)

var _ ActorContext = (*actorContext)(nil)
var _ processor.Unit = (*actorContext)(nil)

type ActorContext interface {
    Logger() log.Logger

    ActorOf(provider ActorProviderFN) ActorGenerator

    ActorOfP(provider ActorProvider) ActorGenerator

    SpawnOf(provider ActorProviderFN) ActorRef

    SpawnOfP(provider ActorProvider) ActorRef

    Tell(target ActorRef, message Message)

    Probe(target ActorRef, message Message)

    // Sender 获取当前正在处理的消息的发送者
    Sender() ActorRef

    // Message 获取当前正在处理的消息
    Message() Message

    //Ask(target ActorRef, message Message, timeout ...time.Duration) Future
}

func newActorContext(system *actorSystem, ref, parent ActorRef, provider ActorProvider, config *ActorConfiguration) *actorContext {
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
    system    *actorSystem        // ActorContext 所属的 ActorSystem
    config    ActorConfiguration  // ActorContext 的配置
    provider  ActorProvider       // ActorContext 的 ActorProvider
    parent    ActorRef            // ActorContext 的父 Actor 的引用，顶级 Actor 的父 Actor 为 nil
    ref       ActorRef            // ActorContext 自身的引用
    mailbox   mailbox.Mailbox     // ActorContext 的邮箱
    childGuid int64               // ActorContext 的子 Actor 的 Guid，用于生成子 Actor 的引用
    children  map[string]ActorRef // ActorContext 的子 Actor 的引用
    actor     Actor               // Actor 实例
    sender    ActorRef            // 当前正在处理的消息的发送者
    message   Message             // 当前正在处理的消息
}

func (ctx *actorContext) HandleUserMessage(sender processor.UnitIdentifier, message any) {
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

func (ctx *actorContext) Sender() ActorRef {
    return ctx.sender
}

func (ctx *actorContext) Message() Message {
    return ctx.message
}

func (ctx *actorContext) onReceive() {
    defer func() {
        if r := recover(); r != nil {
            ctx.Logger().Error("ActorContext OnReceive panic: %v")
        }
    }()
    ctx.actor.Receive(ctx)
}

func (ctx *actorContext) OnSystemMessage(message any) {
    ctx.sender, ctx.message = unwrapMessage(message)
    switch ctx.message.(type) {
    case *OnLaunch:
        ctx.onReceive()
    }
}

func (ctx *actorContext) OnUserMessage(message any) {
    ctx.sender, ctx.message = unwrapMessage(message)
    ctx.onReceive()
}
