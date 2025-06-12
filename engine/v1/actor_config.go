package vivid

import (
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/engine/v1/mailbox"
	"github.com/kercylan98/vivid/src/configurator"
)

// NewActorConfiguration 创建新的Actor配置实例
//
// 支持通过选项模式进行配置，提供灵活的配置方式。
//
// 参数:
//   - options: 可变数量的配置选项
//
// 返回:
//   - *ActorConfiguration: 配置实例
func NewActorConfiguration(options ...ActorOption) *ActorConfiguration {
	c := &ActorConfiguration{}
	for _, option := range options {
		option(c)
	}
	return c
}

type (
	// ActorConfigurator 配置器接口
	ActorConfigurator = configurator.Configurator[*ActorConfiguration]

	// ActorConfiguratorFN 配置器函数类型
	ActorConfiguratorFN = configurator.FN[*ActorConfiguration]

	// ActorOption 配置选项函数类型
	ActorOption = configurator.Option[*ActorConfiguration]

	// ActorConfiguration Actor配置结构体
	//
	// 所有字段均为私有，通过 GetXxx 方法获取值，通过 WithXxx 方法设置值。
	ActorConfiguration struct {
		Name                string                     // Actor 名称
		Logger              log.Logger                 // Actor 日志记录器
		MailboxProvider     mailbox.Provider           // Actor 邮箱提供器
		DispatcherProvider  mailbox.DispatcherProvider // Actor 消息调度器提供器
		SupervisionProvider SupervisorProvider         // Actor 监督者
		PersistenceConfig   *PersistenceConfiguration  // 持久化配置，如果设置则启用持久化功能
	}
)

// WithName 设置 Actor 的名称
func (c *ActorConfiguration) WithName(name string) *ActorConfiguration {
	c.Name = name
	return c
}

// WithActorName 设置 Actor 的名称
func WithActorName(name string) ActorOption {
	return func(c *ActorConfiguration) {
		c.WithName(name)
	}
}

// WithLogger 设置 Actor 的日志记录器
func (c *ActorConfiguration) WithLogger(logger log.Logger) *ActorConfiguration {
	c.Logger = logger
	return c
}

// WithActorLogger 设置 Actor 的日志记录器
func WithActorLogger(logger log.Logger) ActorOption {
	return func(c *ActorConfiguration) {
		c.WithLogger(logger)
	}
}

// WithMailboxProvider 设置 Actor 的邮箱提供器
func (c *ActorConfiguration) WithMailboxProvider(provider mailbox.Provider) *ActorConfiguration {
	c.MailboxProvider = provider
	return c
}

// WithActorMailboxProvider 设置 Actor 的邮箱提供器
func WithActorMailboxProvider(provider mailbox.Provider) ActorOption {
	return func(c *ActorConfiguration) {
		c.WithMailboxProvider(provider)
	}
}

// WithDispatcherProvider 设置 Actor 的消息调度器提供器
func (c *ActorConfiguration) WithDispatcherProvider(provider mailbox.DispatcherProvider) *ActorConfiguration {
	c.DispatcherProvider = provider
	return c
}

// WithActorDispatcherProvider 设置 Actor 的消息调度器提供器
func WithActorDispatcherProvider(provider mailbox.DispatcherProvider) ActorOption {
	return func(c *ActorConfiguration) {
		c.WithDispatcherProvider(provider)
	}
}

// WithSupervisionProvider 设置 Actor 的监督者提供器
func (c *ActorConfiguration) WithSupervisionProvider(provider SupervisorProvider) *ActorConfiguration {
	c.SupervisionProvider = provider
	return c
}

// WithActorSupervisionProvider 设置 Actor 的监督者提供器
func WithActorSupervisionProvider(provider SupervisorProvider) ActorOption {
	return func(c *ActorConfiguration) {
		c.WithSupervisionProvider(provider)
	}
}

// WithPersistenceConfig 设置 Actor 的持久化配置
//
// 当设置了持久化配置时，如果 Actor 实现了 PersistentActor 接口，
// 系统会自动启用持久化功能。
func (c *ActorConfiguration) WithPersistenceConfig(config *PersistenceConfiguration) *ActorConfiguration {
	c.PersistenceConfig = config
	return c
}

// WithActorPersistenceConfig 设置 Actor 的持久化配置
//
// 当设置了持久化配置时，如果 Actor 实现了 PersistentActor 接口，
// 系统会自动启用持久化功能。
func WithActorPersistenceConfig(config *PersistenceConfiguration) ActorOption {
	return func(c *ActorConfiguration) {
		c.WithPersistenceConfig(config)
	}
}
