package vivid

import (
	"context"
	"time"
)

// PersistentActor 定义了支持持久化的 Actor 接口
// Actor 实现此接口后，系统会自动处理持久化恢复流程
type PersistentActor interface {
	Actor

	// PersistenceID 返回此 Actor 的唯一持久化标识符
	// 用于在存储中区分不同的 Actor 实例
	PersistenceID() string

	// Snapshot 获取当前状态的快照
	// 返回的快照数据应该包含恢复状态所需的所有信息
	// 返回 nil 表示当前不需要创建快照
	Snapshot() Message

	// RestoreFromSnapshot 从快照恢复状态
	// 在系统启动时，如果存在快照，会先调用此方法恢复到快照状态
	// 然后再回放快照之后的所有事件
	RestoreFromSnapshot(snapshot Message) error
}

// Event 表示一个持久化事件
type Event struct {
	// PersistenceID Actor 的持久化标识符
	PersistenceID string `json:"persistence_id"`

	// SequenceNumber 事件序列号，用于保证事件顺序
	SequenceNumber int64 `json:"sequence_number"`

	// EventType 事件类型，用于反序列化
	EventType string `json:"event_type"`

	// EventData 事件数据
	EventData Message `json:"event_data"`

	// Timestamp 事件时间戳
	Timestamp time.Time `json:"timestamp"`
}

func newSnapshot(persistenceID string, sequenceNumber int64, snapshotData Message, timestamp time.Time) Snapshot {
	return Snapshot{
		PersistenceID:  persistenceID,
		SequenceNumber: sequenceNumber,
		SnapshotData:   snapshotData,
		Timestamp:      timestamp,
	}
}

// Snapshot 表示一个状态快照
type Snapshot struct {
	// PersistenceID Actor 的持久化标识符
	PersistenceID string `json:"persistence_id"`

	// SequenceNumber 快照对应的最后一个事件序列号
	SequenceNumber int64 `json:"sequence_number"`

	// SnapshotData 快照数据
	SnapshotData Message `json:"snapshot_data"`

	// Timestamp 快照时间戳
	Timestamp time.Time `json:"timestamp"`
}

// PersistenceStore 定义了持久化存储的接口
// 开发者可以实现此接口来使用不同的存储后端（如数据库、文件系统等）
type PersistenceStore interface {
	// SaveEvent 保存事件
	SaveEvent(ctx context.Context, event Event) error

	// LoadEvents 加载指定 Actor 在指定序列号之后的所有事件
	// fromSequenceNumber 为 0 表示加载所有事件
	LoadEvents(ctx context.Context, persistenceID string, fromSequenceNumber int64) ([]Event, error)

	// SaveSnapshot 保存快照
	SaveSnapshot(ctx context.Context, snapshot Snapshot) error

	// LoadSnapshot 加载指定 Actor 的最新快照
	// 如果没有快照，返回 nil
	LoadSnapshot(ctx context.Context, persistenceID string) (*Snapshot, error)

	// GetLastSequenceNumber 获取指定 Actor 的最后一个事件序列号
	// 如果没有事件，返回 0
	GetLastSequenceNumber(ctx context.Context, persistenceID string) (int64, error)

	// OnPersistenceFailed 在生命周期结束或重置时候持续失败将通过该函数执行降级处理
	OnPersistenceFailed(ctx ActorContext, snapshot Snapshot, batch []Event)
}

// EventHandler 定义事件处理回调接口
// 用于在事件持久化成功后处理状态更新
type EventHandler interface {
	// OnEventPersisted 在事件成功持久化后调用
	// event 参数包含已持久化的事件信息
	OnEventPersisted(event Event)
}

// EventHandlerFN 是 EventHandler 接口的函数式实现
// 允许使用函数直接实现事件处理器，简化简单场景的使用
type EventHandlerFN func(event Event)

// OnEventPersisted 实现 EventHandler 接口
func (fn EventHandlerFN) OnEventPersisted(event Event) {
	fn(event)
}

// PersistenceContext 提供持久化相关的功能
// 通过 ActorContext.AsPersistent() 方法获取
type PersistenceContext interface {
	// IsRecovering 返回当前是否处于恢复状态
	// 在恢复期间，Actor 不应该产生副作用（如发送消息、调用外部服务等）
	IsRecovering() bool

	// LastSequenceNumber 返回最后一个事件的序列号
	LastSequenceNumber() int64

	// PersistEvent 持久化一个事件
	// 事件会被缓存，达到阈值时批量持久化，然后调用处理器
	PersistEvent(eventData Message, handler EventHandler) error

	// PersistEventSync 同步持久化一个事件
	// 会立即持久化事件，不使用缓存机制
	PersistEventSync(eventData Message) error

	// MakeSnapshot 制作并保存快照
	// 会调用 Actor 的 Snapshot 方法获取快照数据并保存
	// 快照包含了所有事件的效果，创建快照后可以清理历史事件
	MakeSnapshot() error
}

// PersistenceFailureHandler 定义持久化失败处理接口
// 用户可以在存储层实现此接口来自定义失败处理逻辑
type PersistenceFailureHandler interface {
	// HandlePersistenceFailure 处理持久化失败
	// 当关键生命周期的持久化操作失败时，会调用此方法
	// 用户可以在此方法中实现降级处理、备用存储、通知等逻辑
	HandlePersistenceFailure(ctx ActorContext, snapshot Snapshot, events []Event)
}
