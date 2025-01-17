package vivid

import (
	"fmt"
	"strconv"
)

func generateRootActorContext(system *actorSystem, provider ActorProvider, configurator ...ActorConfigurator) *actorContext {
	config := NewActorConfig(nil)
	systemLoggerProvider := system.config.FetchLoggerProvider()
	config.WithLoggerProvider(systemLoggerProvider)
	for _, c := range configurator {
		c.Configure(config)
	}
	return actorOf(system, nil, provider, config)

}

func actorOf(system *actorSystem, parent *actorContext, provider ActorProvider, config ActorConfiguration) *actorContext {
	config.InitDefault()

	// 生成 Actor 名称
	var name = config.FetchName()
	var parentRef ActorRef
	if parent != nil {
		parentRef = parent.Ref()
		if name == "" {
			parent.childGuid++
			name = string(strconv.AppendInt(nil, parent.childGuid, 10))
		}
	}

	// 初始化内部 Actor 上下文
	internal, err := newInternalActorContext(system, parent, name)
	if err != nil {
		panic(err)
	}

	// 初始化 Actor 上下文
	ctx := &actorContext{
		internalActorContext: internal,
		provider:             provider,
		actor:                provider.Provide(),
		config:               config,
		actorSystem:          system,
		parent:               parentRef,
	}

	// 初始化邮箱
	mailbox := config.FetchMailbox().Provide()
	mailbox.Init(ctx, config.FetchDispatcher().Provide())

	// 绑定子 Actor
	if parent != nil {
		if parent.children == nil {
			parent.children = make(map[string]ActorRef)
		}
		parent.children[ctx.Ref().GetPath()] = ctx.Ref()
	}

	internal.init(ctx, mailbox)
	return ctx
}

func newInternalActorContext(system *actorSystem, parent *actorContext, name string) (*internalActorContext, error) {
	internal := &internalActorContext{}

	if parent != nil {
		internal.ref = parent.internalActorContext.ref.Sub(name)
		_, exist, err := system.processManager.registerProcess(internal)
		if exist {
			return nil, fmt.Errorf("actor [%s] already exists", internal.ref)
		}
		return internal, err
	} else {
		// Root 不注册，设置为守护进程
		internal.ref = system.config.FetchRemoteMessageBuilder().BuildRootID(system.processManager.getHost())
	}

	return internal, nil
}
