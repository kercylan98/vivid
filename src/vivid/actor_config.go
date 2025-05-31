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
	config         *actor.Config
	repository     persistence.Repository // 持久化仓库
	snapshotPolicy *AutoSnapshotPolicy    // 快照策略
	monitoring     Metrics                // 监控指标收集器
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

// WithPersistence 函数将为 Actor 配置持久化功能
//
// 持久化功能提供自动快照管理和状态恢复
//
// 参数:
//   - repository: 持久化仓库
//   - policy: 快照策略，如果为 nil 则使用默认策略
//
// 注意：只有实现了 PersistentActor 接口的 Actor 才能使用持久化功能
func (c *ActorConfig) WithPersistence(repository persistence.Repository, policy *AutoSnapshotPolicy) *ActorConfig {
	c.repository = repository
	c.snapshotPolicy = policy

	if c.snapshotPolicy == nil {
		c.snapshotPolicy = DefaultSnapshotPolicy()
	}

	return c
}

// WithDefaultPersistence 函数将为 Actor 配置默认的持久化功能
//
// 使用默认的快照策略
func (c *ActorConfig) WithDefaultPersistence(repository persistence.Repository) *ActorConfig {
	return c.WithPersistence(repository, nil)
}

// WithSnapshotPolicy 函数设置快照策略
func (c *ActorConfig) WithSnapshotPolicy(policy *AutoSnapshotPolicy) *ActorConfig {
	c.snapshotPolicy = policy
	return c
}

// WithDevelopmentMonitoring 配置开发环境推荐的监控（详细调试）
func (c *ActorConfig) WithDevelopmentMonitoring() *ActorConfig {
	c.monitoring = NewDevelopmentMetrics()
	return c
}

// WithMonitoring 配置Actor监控（实例方法）
func (c *ActorConfig) WithMonitoring(metrics Metrics) *ActorConfig {
	c.monitoring = metrics
	return c
}

// WithDefaultMonitoring 配置默认监控（实例方法）
func (c *ActorConfig) WithDefaultMonitoring() *ActorConfig {
	c.monitoring = NewMetricsCollector(DefaultMonitoringConfig())
	return c
}

// WithSimpleMonitoring 配置简单监控（实例方法）
func (c *ActorConfig) WithSimpleMonitoring() *ActorConfig {
	c.monitoring = NewSimpleMetrics()
	return c
}

// WithProductionMonitoring 配置生产环境推荐的监控（实例方法）
func (c *ActorConfig) WithProductionMonitoring() *ActorConfig {
	c.monitoring = NewProductionMetrics()
	return c
}
