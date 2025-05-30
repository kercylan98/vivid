package vivid

import (
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/src/persistence"
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
)

func newActorConfig() *ActorConfig {
	return &ActorConfig{config: &actor.Config{}}
}

type ActorConfig struct {
	config          *actor.Config
	repository      persistence.Repository // 持久化仓库
	snapshotPolicy  *AutoSnapshotPolicy    // 快照策略
	serializer      Serializer             // 序列化器
	enableSmartMode bool                   // 是否启用智能模式
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
	c.config.Supervisor = actor.SupervisorFN(func(snapshot actor.AccidentSnapshot) {
		supervisor.Decision(newAccidentSnapshot(snapshot))
	})
	return c
}

// WithPersistence 函数将为 Actor 配置持久化仓库（传统模式）
//
// 持久化仓库用于保存和加载 Actor 的状态快照和事件，
// 只有配置了持久化仓库的 Actor 才能使用持久化功能。
//
// 注意：只有实现了 PersistentActor 接口的 Actor 才能真正使用持久化功能。
func (c *ActorConfig) WithPersistence(repository persistence.Repository) *ActorConfig {
	c.repository = repository
	c.enableSmartMode = false
	return c
}

// WithSmartPersistence 函数将为 Actor 配置智能持久化
//
// 智能持久化提供自动快照管理、深拷贝和更好的用户体验
//
// 参数:
//   - repository: 持久化仓库
//   - policy: 快照策略，如果为 nil 则使用默认策略
//   - serializer: 序列化器，如果为 nil 则使用默认的 JSON 序列化器
//
// 注意：只有实现了 SmartPersistentActor 接口的 Actor 才能使用智能持久化功能
func (c *ActorConfig) WithSmartPersistence(repository persistence.Repository, policy *AutoSnapshotPolicy, serializer Serializer) *ActorConfig {
	c.repository = repository
	c.snapshotPolicy = policy
	c.serializer = serializer
	c.enableSmartMode = true

	if c.snapshotPolicy == nil {
		c.snapshotPolicy = DefaultSnapshotPolicy()
	}

	if c.serializer == nil {
		c.serializer = &JSONSerializer{}
	}

	return c
}

// WithDefaultSmartPersistence 函数将为 Actor 配置默认的智能持久化
//
// 使用默认的快照策略和 JSON 序列化器
func (c *ActorConfig) WithDefaultSmartPersistence(repository persistence.Repository) *ActorConfig {
	return c.WithSmartPersistence(repository, nil, nil)
}

// WithSnapshotPolicy 函数设置快照策略（仅在智能模式下有效）
func (c *ActorConfig) WithSnapshotPolicy(policy *AutoSnapshotPolicy) *ActorConfig {
	c.snapshotPolicy = policy
	return c
}

// WithSerializer 函数设置序列化器（仅在智能模式下有效）
func (c *ActorConfig) WithSerializer(serializer Serializer) *ActorConfig {
	c.serializer = serializer
	return c
}
