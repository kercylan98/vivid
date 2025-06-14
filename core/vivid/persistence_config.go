package vivid

import (
	"time"

	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/src/configurator"
	"github.com/kercylan98/vivid/src/serializer"
)

// NewPersistenceConfiguration 创建新的持久化配置实例。
//
// 支持通过选项模式进行配置，提供灵活的配置方式。
//
// 参数:
//   - options: 可变数量的配置选项
//
// 返回:
//   - *PersistenceConfiguration: 配置实例
func NewPersistenceConfiguration(options ...PersistenceOption) *PersistenceConfiguration {
	c := &PersistenceConfiguration{
		Logger:             log.GetDefault(),
		SnapshotInterval:   100,
		EnableAutoSnapshot: true,
		EventBatchSize:     10,
		EventFlushInterval: time.Second,
	}
	for _, option := range options {
		option(c)
	}
	return c
}

type (
	// PersistenceConfigurator 配置器接口
	PersistenceConfigurator = configurator.Configurator[*PersistenceConfiguration]

	// PersistenceConfiguratorFN 配置器函数类型
	PersistenceConfiguratorFN = configurator.FN[*PersistenceConfiguration]

	// PersistenceOption 配置选项函数类型
	PersistenceOption = configurator.Option[*PersistenceConfiguration]

	// PersistenceConfiguration 持久化配置结构体。
	//
	// 包含持久化系统运行所需的所有配置参数。
	// 所有字段均为私有，通过 GetXxx 方法获取值，通过 WithXxx 方法设置值。
	PersistenceConfiguration struct {
		// Logger 日志记录器，用于记录持久化操作的日志信息。
		// 包括事件写入、快照创建、恢复过程等运行时日志。
		// 默认值：log.GetDefault()
		// 注意：建议在生产环境中使用结构化日志记录器以便于问题排查
		Logger log.Logger

		// Store 持久化存储实例，用于实际的数据存储操作。
		// 必填项：此字段必须设置，否则持久化功能无法正常工作
		// 注意：不同的存储实现可能有不同的性能特征和一致性保证
		Store PersistenceStore

		// Serializer 序列化器，用于事件和快照的序列化。
		// 负责将事件和快照对象转换为字节流进行存储，以及反向转换。
		// 必填项：此字段必须设置，否则无法正确处理事件和快照
		// 注意：序列化器的选择会影响性能和兼容性，建议选择高效且向后兼容的格式
		Serializer serializer.NameSerializer

		// SnapshotInterval 自动快照间隔。
		// 当事件数量达到此间隔时，自动创建快照以优化恢复性能。
		// 默认值：100 个事件
		SnapshotInterval int64

		// EnableAutoSnapshot 是否启用自动快照。
		// 启用后，系统会根据 SnapshotInterval 自动创建快照。
		// 默认值：true
		EnableAutoSnapshot bool

		// EventBatchSize 事件批量处理大小。
		// 当缓存的事件数量达到此值时，会触发批量持久化。
		// 默认值：10
		EventBatchSize int

		// EventFlushInterval 事件刷新间隔。
		// 即使未达到批量大小，也会定期刷新缓存的事件。
		// 默认值：1 秒
		EventFlushInterval time.Duration
	}
)

// WithLogger 设置日志记录器。
// 该方法返回配置实例本身，支持链式调用。
// logger 参数指定要使用的日志记录器。
func (c *PersistenceConfiguration) WithLogger(logger log.Logger) *PersistenceConfiguration {
	c.Logger = logger
	return c
}

// WithPersistenceLogger 创建设置日志记录器的配置选项。
// 返回一个可用于 NewPersistenceConfiguration 的配置选项函数。
// logger 参数指定要设置的日志记录器。
func WithPersistenceLogger(logger log.Logger) PersistenceOption {
	return func(c *PersistenceConfiguration) {
		c.WithLogger(logger)
	}
}

// WithStore 设置持久化存储实例。
// 该方法返回配置实例本身，支持链式调用。
// store 参数指定要使用的存储实例。
func (c *PersistenceConfiguration) WithStore(store PersistenceStore) *PersistenceConfiguration {
	c.Store = store
	return c
}

// WithPersistenceStore 创建设置存储实例的配置选项。
// 返回一个可用于 NewPersistenceConfiguration 的配置选项函数。
// store 参数指定要设置的存储实例。
func WithPersistenceStore(store PersistenceStore) PersistenceOption {
	return func(c *PersistenceConfiguration) {
		c.WithStore(store)
	}
}

// WithSerializer 设置序列化器。
// 该方法返回配置实例本身，支持链式调用。
// serializer 参数指定要使用的序列化器。
func (c *PersistenceConfiguration) WithSerializer(serializer serializer.NameSerializer) *PersistenceConfiguration {
	c.Serializer = serializer
	return c
}

// WithPersistenceSerializer 创建设置序列化器的配置选项。
// 返回一个可用于 NewPersistenceConfiguration 的配置选项函数。
// serializer 参数指定要设置的序列化器。
func WithPersistenceSerializer(serializer serializer.NameSerializer) PersistenceOption {
	return func(c *PersistenceConfiguration) {
		c.WithSerializer(serializer)
	}
}

// WithSnapshotInterval 设置自动快照间隔。
// 该方法返回配置实例本身，支持链式调用。
// interval 参数指定快照间隔的事件数量。
func (c *PersistenceConfiguration) WithSnapshotInterval(interval int64) *PersistenceConfiguration {
	c.SnapshotInterval = interval
	return c
}

// WithPersistenceSnapshotInterval 创建设置快照间隔的配置选项。
// 返回一个可用于 NewPersistenceConfiguration 的配置选项函数。
// interval 参数指定要设置的快照间隔。
func WithPersistenceSnapshotInterval(interval int64) PersistenceOption {
	return func(c *PersistenceConfiguration) {
		c.WithSnapshotInterval(interval)
	}
}

// WithEnableAutoSnapshot 设置是否启用自动快照。
// 该方法返回配置实例本身，支持链式调用。
// enable 参数指定是否启用自动快照。
func (c *PersistenceConfiguration) WithEnableAutoSnapshot(enable bool) *PersistenceConfiguration {
	c.EnableAutoSnapshot = enable
	return c
}

// WithPersistenceEnableAutoSnapshot 创建设置自动快照开关的配置选项。
// 返回一个可用于 NewPersistenceConfiguration 的配置选项函数。
// enable 参数指定是否启用自动快照。
func WithPersistenceEnableAutoSnapshot(enable bool) PersistenceOption {
	return func(c *PersistenceConfiguration) {
		c.WithEnableAutoSnapshot(enable)
	}
}

// WithEventBatchSize 设置事件批量处理大小。
// 该方法返回配置实例本身，支持链式调用。
// batchSize 参数指定批量处理的事件数量。
func (c *PersistenceConfiguration) WithEventBatchSize(batchSize int) *PersistenceConfiguration {
	c.EventBatchSize = batchSize
	return c
}

// WithPersistenceEventBatchSize 创建设置事件批量大小的配置选项。
// 返回一个可用于 NewPersistenceConfiguration 的配置选项函数。
// batchSize 参数指定要设置的批量大小。
func WithPersistenceEventBatchSize(batchSize int) PersistenceOption {
	return func(c *PersistenceConfiguration) {
		c.WithEventBatchSize(batchSize)
	}
}

// WithEventFlushInterval 设置事件刷新间隔。
// 该方法返回配置实例本身，支持链式调用。
// interval 参数指定事件刷新的时间间隔。
func (c *PersistenceConfiguration) WithEventFlushInterval(interval time.Duration) *PersistenceConfiguration {
	c.EventFlushInterval = interval
	return c
}

// WithPersistenceEventFlushInterval 创建设置事件刷新间隔的配置选项。
// 返回一个可用于 NewPersistenceConfiguration 的配置选项函数。
// interval 参数指定要设置的刷新间隔。
func WithPersistenceEventFlushInterval(interval time.Duration) PersistenceOption {
	return func(c *PersistenceConfiguration) {
		c.WithEventFlushInterval(interval)
	}
}
