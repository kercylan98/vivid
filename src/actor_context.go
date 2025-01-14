package vivid

import (
	"fmt"
	"strconv"
)

var (
	_ ActorContext    = (*actorContext)(nil)    // 确保 actorContext 实现了 ActorContext 接口
	_ ActorSpawnChain = (*actorSpawnChain)(nil) // 确保 actorSpawnChain 实现了 ActorSpawnChain 接口
)

// ActorContext 是定义了 Actor 完整的上下文。
type ActorContext interface {
	ActorContextSpawner
	ActorContextLogger
	ActorContextLife
	ActorContextTransport
}

// ActorContextSpawner 是 ActorContext 的子集，它确保了对子 Actor 的生成
//   - 所有的生成函数均无法保证并发安全
type ActorContextSpawner interface {
	// ActorOf 生成 Actor
	ActorOf(provider ActorProvider, configurator ...ActorConfigurator) ActorRef

	// ActorOfFn 函数式生成 Actor
	ActorOfFn(provider ActorProviderFn, configurator ...ActorConfiguratorFn) ActorRef

	// ActorOfConfig 生成 Actor
	ActorOfConfig(provider ActorProvider, config ActorConfiguration) ActorRef

	// ChainOf 通过责任链的方式生成 Actor
	ChainOf(provider ActorProvider) ActorSpawnChain
}

// ActorContextLogger 是 ActorContext 的子集，它确保了对日志的记录
type ActorContextLogger interface {
	// Logger 获取日志记录器
	Logger() *Logger

	// GetLoggerFetcher 获取日志记录器提供者
	GetLoggerFetcher() LoggerFetcher
}

// ActorContextLife 是 ActorContext 的子集，它确保了对 Actor 生命周期的控制
type ActorContextLife interface {
	// Ref 获取当前 Actor 的 ActorRef
	Ref() ActorRef

	// Parent 获取父 Actor 的 ActorRef
	Parent() ActorRef
}

// ActorContextTransport 是 ActorContext 的子集，它确保了对 Actor 之间的通信
type ActorContextTransport interface {
	// Sender 获取当前消息的发送者
	Sender() ActorRef

	// Message 获取当前消息的内容
	Message() Message
}

// ActorSpawnChain 是 Actor 生成链，用于生成 Actor
type ActorSpawnChain interface {
	// SetConfig 设置 ActorConfiguration
	SetConfig(config ActorConfiguration) ActorSpawnChain

	// SetConfigurator 设置 ActorConfigurator
	SetConfigurator(configurator ActorConfigurator) ActorSpawnChain

	// SetFnConfigurator 设置 ActorConfiguratorFn
	SetFnConfigurator(configurator ActorConfiguratorFn) ActorSpawnChain

	// AddNextConfigurator 添加 ActorConfigurator
	AddNextConfigurator(configurator ActorConfigurator) ActorSpawnChain

	// AddNextFnConfigurator 添加 ActorConfiguratorFn
	AddNextFnConfigurator(configurator ActorConfiguratorFn) ActorSpawnChain

	// ActorOf 生成 Actor
	ActorOf() ActorRef
}

func newActorSpawnChain(parent ActorContext, provider ActorProvider) ActorSpawnChain {
	return &actorSpawnChain{
		parent:   parent,
		provider: provider,
	}
}

type actorSpawnChain struct {
	parent        ActorContext
	provider      ActorProvider
	config        ActorConfiguration
	configurators []ActorConfigurator
}

func (a *actorSpawnChain) SetConfig(config ActorConfiguration) ActorSpawnChain {
	a.config = config
	return a
}

func (a *actorSpawnChain) SetConfigurator(configurator ActorConfigurator) ActorSpawnChain {
	a.configurators = []ActorConfigurator{configurator}
	return a
}

func (a *actorSpawnChain) SetFnConfigurator(configurator ActorConfiguratorFn) ActorSpawnChain {
	return a.SetConfigurator(configurator)
}

func (a *actorSpawnChain) AddNextConfigurator(configurator ActorConfigurator) ActorSpawnChain {
	a.configurators = append(a.configurators, configurator)
	return a
}

func (a *actorSpawnChain) AddNextFnConfigurator(configurator ActorConfiguratorFn) ActorSpawnChain {
	return a.AddNextConfigurator(configurator)
}

func (a *actorSpawnChain) ActorOf() ActorRef {
	if a.config == nil {
		a.config = NewActorConfig(a.parent)
	}
	for _, configurator := range a.configurators {
		configurator.Configure(a.config)
	}
	return a.parent.ActorOfConfig(a.provider, a.config)
}

type actorContext struct {
	*internalActorContext                    // 内部 Actor 上下文
	provider              ActorProvider      // Actor 提供者
	actor                 Actor              // Actor 实例
	config                ActorConfiguration // Actor 配置
	actorSystem           *actorSystem       // 所属的 ActorSystem
	childGuid             int64              // 子 Actor GUID
	children              map[ActorRef]Actor // 子 Actor
	root                  bool               // 是否是根 Actor
	parent                ActorRef           // 父 Actor
	envelope              Envelope           // 当前消息
}

func (a *actorContext) Sender() ActorRef {
	if a.envelope == nil {
		return nil
	}
	return a.envelope.GetSender()
}

func (a *actorContext) Message() Message {
	if a.envelope == nil {
		return nil
	}
	return a.envelope.GetMessage()
}

func (a *actorContext) Ref() ActorRef {
	return a.ref
}

func (a *actorContext) Parent() ActorRef {
	return a.parent
}

func (a *actorContext) GetLoggerFetcher() LoggerFetcher {
	return a.config.FetchLoggerFetcher()
}

func (a *actorContext) Logger() *Logger {
	return a.config.FetchLogger()
}

func (a *actorContext) ActorOf(provider ActorProvider, configurator ...ActorConfigurator) ActorRef {
	config := NewActorConfig(a)
	for _, c := range configurator {
		c.Configure(config)
	}
	return a.ActorOfConfig(provider, config)
}

func (a *actorContext) ActorOfFn(provider ActorProviderFn, configurator ...ActorConfiguratorFn) ActorRef {
	var c = make([]ActorConfigurator, len(configurator))
	for i, f := range configurator {
		c[i] = f
	}
	return a.ActorOf(provider, c...)
}

func (a *actorContext) ChainOf(provider ActorProvider) ActorSpawnChain {
	return newActorSpawnChain(a, provider)
}

func (a *actorContext) ActorOfConfig(provider ActorProvider, config ActorConfiguration) ActorRef {
	ctx := actorOf(a.actorSystem, a, provider, config)
	return ctx.Ref()
}

func generateRootActorContext(system *actorSystem, provider ActorProvider, configurator ...ActorConfigurator) *actorContext {
	config := NewActorConfig(nil)
	config.WithLoggerFetcher(system.GetLoggerFetcher())
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
		children:             make(map[ActorRef]Actor),
		parent:               parentRef,
	}

	// 初始化邮箱
	mailbox := config.FetchMailbox().Provide()
	mailbox.Init(ctx, config.FetchDispatcher().Provide())

	internal.init(ctx, mailbox)
	return ctx
}

func newInternalActorContext(system *actorSystem, parent *actorContext, name string) (*internalActorContext, error) {
	internal := &internalActorContext{}

	if parent != nil {
		internal.ref = parent.internalActorContext.ref.Sub(name)
	} else {
		internal.ref = GetIDBuilder().RootOf(system.processManager.getHost())
	}

	_, exist, err := system.processManager.registerProcess(internal)
	if exist {
		return nil, fmt.Errorf("actor [%s] already exists", internal.ref)
	}

	return internal, err
}
