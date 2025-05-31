package vivid

import (
	"time"

	"github.com/kercylan98/vivid/src/persistence"
)

// PersistentActor 持久化Actor接口
//
// 系统会自动处理快照、恢复和事件重放等复杂逻辑
type PersistentActor interface {
	Actor

	// GetPersistenceId 返回此Actor的持久化标识符
	GetPersistenceId() string

	// GetCurrentState 获取当前状态，用于自动快照
	//
	// 返回的状态将被系统保存为快照
	// 注意：repository实现需要处理状态的序列化
	GetCurrentState() any

	// RestoreState 恢复状态到指定的状态
	//
	// 系统会在Actor启动时自动调用此方法来恢复状态
	// 用户只需要将传入的状态设置到Actor的字段中即可
	RestoreState(state any)
}

// PersistenceContext 持久化操作的上下文接口
//
// 提供持久化操作的能力，系统会自动处理快照策略
type PersistenceContext interface {
	// Persist 持久化一个事件
	//
	// 系统会根据配置的策略自动决定是否创建快照
	Persist(event persistence.Event) error

	// ForceSnapshot 强制创建快照
	//
	// 一般不需要手动调用，系统会根据策略自动创建快照
	ForceSnapshot() error
}

// AutoSnapshotPolicy 自动快照策略配置
type AutoSnapshotPolicy struct {
	// EventThreshold 事件数量阈值，超过此数量自动创建快照
	EventThreshold int
	// TimeThreshold 时间阈值，超过此时间自动创建快照
	TimeThreshold time.Duration
	// StateChangeThreshold 状态变化阈值，状态变化超过此比例时创建快照
	StateChangeThreshold float64
	// ForceSnapshotOnShutdown 在Actor关闭时强制创建快照
	ForceSnapshotOnShutdown bool
}

// DefaultSnapshotPolicy 返回默认的快照策略
func DefaultSnapshotPolicy() *AutoSnapshotPolicy {
	return &AutoSnapshotPolicy{
		EventThreshold:          10,              // 每10个事件创建一次快照
		TimeThreshold:           5 * time.Minute, // 每5分钟创建一次快照
		StateChangeThreshold:    0.3,             // 状态变化30%时创建快照
		ForceSnapshotOnShutdown: true,
	}
}

// ShouldCreateSnapshot 判断是否应该创建快照
func (p *AutoSnapshotPolicy) ShouldCreateSnapshot(eventCount int, lastSnapshot time.Time) bool {
	// 基于事件数量判断
	if eventCount >= p.EventThreshold {
		return true
	}

	// 基于时间判断
	if time.Since(lastSnapshot) >= p.TimeThreshold {
		return true
	}

	return false
}

// persistenceContext 持久化上下文的简化实现
type persistenceContext struct {
	state        *persistence.State
	policy       *AutoSnapshotPolicy
	actor        PersistentActor
	eventCount   int
	lastSnapshot time.Time
}

// newPersistenceContext 创建持久化上下文
func newPersistenceContext(state *persistence.State, policy *AutoSnapshotPolicy, actor PersistentActor) PersistenceContext {
	if policy == nil {
		policy = DefaultSnapshotPolicy()
	}

	return &persistenceContext{
		state:        state,
		policy:       policy,
		actor:        actor,
		lastSnapshot: time.Now(),
	}
}

func (p *persistenceContext) Persist(event persistence.Event) error {
	// 持久化事件
	p.state.Update(event)
	p.eventCount++

	// 检查是否需要自动创建快照
	if p.policy.ShouldCreateSnapshot(p.eventCount, p.lastSnapshot) {
		if err := p.ForceSnapshot(); err != nil {
			return err
		}
	}

	// 保存到仓库
	return p.state.Persist()
}

func (p *persistenceContext) ForceSnapshot() error {
	// 获取当前状态
	currentState := p.actor.GetCurrentState()

	// 直接保存状态，不进行深拷贝
	// 深拷贝的责任应该在repository层根据具体的序列化需求处理
	p.state.SaveSnapshot(currentState)
	p.eventCount = 0 // 重置事件计数
	p.lastSnapshot = time.Now()

	return p.state.Persist()
}

// autoRecover 自动恢复Actor状态
func autoRecover(state *persistence.State, actor PersistentActor) error {
	if err := state.Load(); err != nil {
		return err
	}

	// 从快照恢复状态
	if snapshot := state.GetSnapshot(); snapshot != nil {
		actor.RestoreState(snapshot)
	}

	// 注意：事件重放在这个简化版本中暂不实现
	// 因为需要复杂的消息重发机制
	return nil
}
