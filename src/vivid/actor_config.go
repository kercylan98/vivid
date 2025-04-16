package vivid

import (
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
)

func newActorConfig() *ActorConfig {
	return &ActorConfig{config: &actor.Config{}}
}

type ActorConfig struct {
	config *actor.Config
}

// WithDispatcher 函数将使用指定的调度器来创建 Actor
//
// 调度器是邮箱消息的处理器，邮箱消息在取出后会交给调度器进行处理。
func (c *ActorConfig) WithDispatcher(dispatcher Dispatcher) *ActorConfig {
	c.config.Dispatcher = dispatcher
	return c
}

// WithMailbox 函数将使用指定的邮箱来创建 Actor
//
// 邮箱是用于存储 Actor 消息的对象，Actor 将从邮箱中获取消息进行处理。
// 邮箱的实现可以是 FIFO、LIFO、Priority 等等。
// 具体的实现可以参考 Mailbox 接口。
// 该函数将覆盖默认的邮箱实现。
func (c *ActorConfig) WithMailbox(mailbox Mailbox) *ActorConfig {
	c.config.Mailbox = mailbox
	return c
}

// WithName 函数将以特定的名称创建 Actor，它将被广泛的用于日志、唯一性等场景中
func (c *ActorConfig) WithName(name string) *ActorConfig {
	c.config.Name = name
	return c
}

// WithLoggerProvider 函数将使用指定的日志提供器来创建 Actor
func (c *ActorConfig) WithLoggerProvider(provider log.Provider) *ActorConfig {
	c.config.LoggerProvider = provider
	return c
}

// WithSupervisor 函数将使用指定的监管者来创建 Actor
//
// 监管者是用于在 Actor 发生异常时对其执行监管策略的对象
func (c *ActorConfig) WithSupervisor(supervisor Supervisor) *ActorConfig {
	c.config.Supervisor = actor.SupervisorFN(func(snapshot actor.Snapshot) {
		supervisor.Decision(newAccidentSnapshot(snapshot))
	})
	return c
}
