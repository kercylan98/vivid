package vivid

import (
	"fmt"
	"github.com/kercylan98/go-log/log"
	"strconv"
	"time"
)

var (
	_ actorContextSpawnerInternal = (*actorContextSpawnerImpl)(nil)
)

func newActorContextSpawnerImpl(ctx ActorContext, provider ActorProvider) *actorContextSpawnerImpl {
	return &actorContextSpawnerImpl{
		ActorContext: ctx,
		provider:     provider,
		actor:        provider.Provide(),
	}
}

type actorContextSpawnerImpl struct {
	ActorContext
	provider ActorProvider // Actor 提供者
	actor    Actor         // Actor 实例
}

func (ctx *actorContextSpawnerImpl) getActor() Actor {
	return ctx.actor
}

func (ctx *actorContextSpawnerImpl) ActorOf(provider ActorProvider, configurator ...ActorConfigurator) ActorRef {
	config := NewActorConfig(ctx)
	for _, c := range configurator {
		c.Configure(config)
	}
	return ctx.ActorOfConfig(provider, config)
}

func (ctx *actorContextSpawnerImpl) ActorOfFn(provider ActorProviderFn, configurator ...ActorConfiguratorFn) ActorRef {
	var c = make([]ActorConfigurator, len(configurator))
	for i, f := range configurator {
		c[i] = f
	}
	return ctx.ActorOf(provider, c...)
}

func (ctx *actorContextSpawnerImpl) ChainOf(provider ActorProvider) ActorSpawnChain {
	return newActorSpawnChain(ctx, provider)
}

func (ctx *actorContextSpawnerImpl) ActorOfConfig(provider ActorProvider, config ActorConfiguration) ActorRef {
	return actorOf(ctx.System(), ctx, provider, config).Ref()
}

func generateRootActorContext(system ActorSystem, provider ActorProvider, configurator ...ActorConfigurator) ActorContext {
	config := NewActorConfig(nil)
	systemLoggerProvider := system.getConfig().FetchLoggerProvider()
	config.WithLoggerProvider(systemLoggerProvider)
	for _, c := range configurator {
		c.Configure(config)
	}
	return actorOf(system, nil, provider, config)

}

func actorOf(system ActorSystem, parent ActorContext, provider ActorProvider, config ActorConfiguration) ActorContext {
	config.InitDefault()

	// 生成 Actor 名称
	var name = config.FetchName()
	var parentRef ActorRef
	if parent != nil {
		parentRef = parent.Ref()
		if name == "" {
			name = string(strconv.AppendInt(nil, parent.getNextChildGuid(), 10))
		}
	}

	// 初始化 ActorRef
	var ref ActorRef
	if parent != nil {
		// 注册进程
		ref = parent.Ref().Sub(name)
	} else {
		// Root 不注册，设置为守护进程
		ref = system.getConfig().FetchRemoteMessageBuilder().BuildRootID(system.getProcessManager().getHost())
	}

	// 初始化邮箱及上下文
	mailbox := config.FetchMailbox().Provide()
	ctx := newActorContext(system, config, provider, mailbox, ref, parentRef)
	mailbox.Init(ctx.recipient, config.FetchDispatcher().Provide())

	// 注册进程
	if _, exist, err := system.getProcessManager().registerProcess(ctx.getProcess()); err != nil {
		panic(err)
	} else if exist {
		panic(fmt.Errorf("actor [%s] already exists", ref))
	}

	// 绑定子 Actor
	if parent != nil {
		parent.bindChild(ctx.Ref())
	}

	// 初始化启动上下文
	var launchContext map[any]any
	if launchContextProvider := ctx.getConfig().FetchLaunchContextProvider(); launchContextProvider != nil {
		launchContext = launchContextProvider.Provide()
	}

	// 启动完成
	ctx.Tell(ctx.Ref(), newOnLaunch(time.Now(), launchContext))
	if parent != nil {
		ctx.Logger().Debug("ActorSpawn", log.String("actor", ctx.Ref().String()))
	} else {
		ctx.System().startingLog(log.String("stage", "guard"), log.String("info", "guard actor initialized"))
	}

	return ctx
}
