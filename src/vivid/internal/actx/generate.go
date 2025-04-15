package actx

import (
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
	"github.com/kercylan98/vivid/src/vivid/internal/mailbox"
	"github.com/kercylan98/vivid/src/vivid/internal/ref"
	"strconv"
)

var _ actor.GenerateContext = (*Generate)(nil)

func NewGenerate(ctx actor.Context, provider actor.Provider) *Generate {
	return &Generate{
		ctx:      ctx,
		provider: provider,
	}
}

type Generate struct {
	ctx      actor.Context
	provider actor.Provider
	actor    actor.Actor
}

func (g *Generate) Actor() actor.Actor {
	return g.actor
}

func (g *Generate) ResetActorState() {
	g.actor = g.provider.Provide()
}

func (g *Generate) GenerateActorContext(system actor.System, parent actor.Context, provider actor.Provider, config actor.Config) actor.Context {
	// 预设日志提供器
	if config.LoggerProvider == nil {
		if parent == nil {
			config.LoggerProvider = system.LoggerProvider()
		} else {
			config.LoggerProvider = parent.LoggerProvider()
		}
	}

	if config.Mailbox == nil {
		config.Mailbox = mailbox.NewMailbox()
	}

	if config.Dispatcher == nil {
		config.Dispatcher = mailbox.NewDispatcher()
	}

	// 预设名称
	var parentRef actor.Ref
	if parent != nil {
		parentRef = parent.MetadataContext().Ref()
		if config.Name == "" {
			config.Name = string(strconv.AppendInt(nil, parent.RelationContext().NextGuid(), 10))
		}
	}

	// 引用初始化
	var actorRef actor.Ref
	if parentRef != nil {
		actorRef = parentRef.GenerateSub(config.Name)
	} else {
		actorRef = ref.NewActorRef(system.ResourceLocator())
	}

	// 初始化邮箱及上下文
	ctx := New(system, &config, actorRef, parentRef, provider)
	ctx.GenerateContext().ResetActorState()
	config.Mailbox.Initialize(config.Dispatcher, ctx.MessageContext())
	if parent != nil {
		system.Register(ctx)
		parent.RelationContext().BindChild(actorRef)
	}

	// 投递启动消息
	if parent != nil {
		parent.TransportContext().Tell(actorRef, SystemMessage, actor.OnLaunchMessageInstance)
	}

	return ctx
}
