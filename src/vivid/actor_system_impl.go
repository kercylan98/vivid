package vivid

import (
	"fmt"
	"github.com/kercylan98/wasteland/src/wasteland"
	"strconv"
)

var (
	_ ActorSystem = (*actorSystemImpl)(nil)
)

func newActorSystem(config ActorSystemConfiguration) ActorSystem {
	return &actorSystemImpl{
		config: &config, // 采用引用，避免内部使用过程中拷贝
	}
}

type actorSystemImpl struct {
	config          *ActorSystemConfiguration // 配置
	processRegistry wasteland.ProcessRegistry // 进程注册表
}

func (sys *actorSystemImpl) actorOf(parent ActorContext, provider ActorProvider, config ActorConfiguration) ActorContext {
	// 预设初始化
	if config.ActorRuntimeConfiguration.LoggerProvider == nil {
		config.ActorRuntimeConfiguration.LoggerProvider = sys.config.LoggerProvider
	}

	// 初始化名称
	var name = config.Name
	var parentRef ActorRef
	if parent != nil {
		parentRef = parent.Ref()
		if name == "" {
			if children, cast := parent.(actorContextChildren); cast {
				name = string(strconv.AppendInt(nil, children.nextGuid(), 10))
			} else {
				panic(fmt.Errorf("parent actor context %T does not implements actorContextChildren", parent))
			}
		}
	}

	// 初始化引用
	var ref ActorRef
	if parentRef != nil {
		if generator, cast := parentRef.(actorRefProcessInfo); cast {
			ref = generator.generateSub(name)
		} else {
			panic(fmt.Errorf("parent actor ref %T does not implements actorRefProcessInfo", parentRef))
		}
	} else {
		ref = newActorRef(wasteland.NewProcessId(sys.processRegistry.Meta(), ""))
	}

	// 初始化邮箱及上下文
	mailbox := config.Mailbox
	dispatcher := config.Dispatcher
	ctx := newActorContext(sys, ref, parentRef, &config)
	mailbox.Initialize(dispatcher, ctx)

	// 注册进程
	if err := sys.processRegistry.Register(ctx); err != nil {
		panic(err)
	}

	// 绑定父子关系
	if parent != nil {
		parent.bindChild(ref)
	}

	// 启动完成
	return ctx
}

func (sys *actorSystemImpl) getConfig() *ActorSystemConfiguration {
	return sys.config
}

// Start 启动 Actor 系统
func (sys *actorSystemImpl) Start() error {
	sys.processRegistry = wasteland.NewProcessRegistry(wasteland.ProcessRegistryConfig{
		Meta:          wasteland.NewMeta(sys.config.Address),
		Daemon:        nil,
		LoggerProvide: sys.config.LoggerProvider,
	})

	return sys.processRegistry.Run()
}

// StartP 启动 Actor 系统，并在发生异常时 panic
func (sys *actorSystemImpl) StartP() ActorSystem {
	if err := sys.Start(); err != nil {
		panic(err)
	}
	return sys
}

// Shutdown 关闭 Actor 系统
func (sys *actorSystemImpl) Shutdown() error {
	sys.processRegistry.Stop()
	return nil
}

// ShutdownP 关闭 Actor 系统，并在发生异常时 panic
func (sys *actorSystemImpl) ShutdownP() ActorSystem {
	if err := sys.Shutdown(); err != nil {
		panic(err)
	}
	return sys
}
