package vivid

import (
	"time"

	"github.com/kercylan98/vivid/src/persistence"
)

// SmartPersistentActor 是增强版的持久化 Actor 接口
//
// 相比 PersistentActor，它提供了更智能的持久化功能和更好的用户体验
type SmartPersistentActor interface {
	Actor

	// OnRecover 当 Actor 从持久化存储中恢复时被调用
	OnRecover(ctx SmartPersistenceContext)

	// GetPersistenceId 返回此 Actor 的持久化标识符
	GetPersistenceId() string

	// GetCurrentState 获取当前状态，用于智能快照
	//
	// 返回的状态将被自动深拷贝用于快照保存
	GetCurrentState() any

	// ApplyEvent 应用事件到当前状态
	//
	// 这个方法用于在事件重放时更新状态
	ApplyEvent(event persistence.Event)
}

// PersistentActor 是支持持久化的 Actor 接口（保持向后兼容）
type PersistentActor interface {
	Actor

	// OnRecover 当 Actor 从持久化存储中恢复时被调用
	OnRecover(ctx PersistenceContext)

	// GetPersistenceId 返回此 Actor 的持久化标识符
	GetPersistenceId() string
}

// SmartPersistenceContext 是增强版的持久化上下文接口
type SmartPersistenceContext interface {
	PersistenceContext

	// PersistWithState 持久化事件并自动管理快照
	//
	// 该方法会根据配置的策略自动决定是否创建快照
	PersistWithState(event persistence.Event, currentState any) error

	// ForceSnapshot 强制创建快照
	ForceSnapshot(state any) error

	// GetSnapshotPolicy 获取当前的快照策略
	GetSnapshotPolicy() *AutoSnapshotPolicy

	// SetSnapshotPolicy 设置快照策略
	SetSnapshotPolicy(policy *AutoSnapshotPolicy)

	// GetEventCount 获取自上次快照以来的事件数量
	GetEventCount() int

	// GetLastSnapshotTime 获取上次快照时间
	GetLastSnapshotTime() time.Time
}

// PersistenceContext 是持久化操作的上下文接口（保持向后兼容）
type PersistenceContext interface {
	// GetSnapshot 获取当前的快照数据
	GetSnapshot() persistence.Snapshot

	// GetEvents 获取自最后一次快照以来的所有事件
	GetEvents() []persistence.Event

	// Persist 持久化一个事件
	Persist(event persistence.Event)

	// SaveSnapshot 保存当前状态的快照
	SaveSnapshot(snapshot persistence.Snapshot)

	// CanRecover 检查是否有可恢复的数据
	CanRecover() bool
}

// smartPersistenceContext 是增强版持久化上下文的实现
type smartPersistenceContext struct {
	manager *SmartPersistenceManager
}

// newSmartPersistenceContext 创建增强版持久化上下文
func newSmartPersistenceContext(manager *SmartPersistenceManager) SmartPersistenceContext {
	return &smartPersistenceContext{
		manager: manager,
	}
}

func (p *smartPersistenceContext) GetSnapshot() persistence.Snapshot {
	return p.manager.state.GetSnapshot()
}

func (p *smartPersistenceContext) GetEvents() []persistence.Event {
	return p.manager.state.GetEvents()
}

func (p *smartPersistenceContext) Persist(event persistence.Event) {
	p.manager.state.Update(event)
	p.manager.state.Persist()
}

func (p *smartPersistenceContext) SaveSnapshot(snapshot persistence.Snapshot) {
	p.manager.state.SaveSnapshot(snapshot)
	p.manager.state.Persist()
}

func (p *smartPersistenceContext) CanRecover() bool {
	return p.manager.state.GetSnapshot() != nil || len(p.manager.state.GetEvents()) > 0
}

func (p *smartPersistenceContext) PersistWithState(event persistence.Event, currentState any) error {
	return p.manager.PersistEvent(event, currentState)
}

func (p *smartPersistenceContext) ForceSnapshot(state any) error {
	return p.manager.ForceSnapshot(state)
}

func (p *smartPersistenceContext) GetSnapshotPolicy() *AutoSnapshotPolicy {
	return p.manager.policy
}

func (p *smartPersistenceContext) SetSnapshotPolicy(policy *AutoSnapshotPolicy) {
	p.manager.policy = policy
}

func (p *smartPersistenceContext) GetEventCount() int {
	return p.manager.eventCount
}

func (p *smartPersistenceContext) GetLastSnapshotTime() time.Time {
	return p.manager.lastSnapshot
}

// persistenceContext 是原始持久化上下文的实现（保持向后兼容）
type persistenceContext struct {
	state *persistence.State
}

// newPersistenceContext 创建持久化上下文
func newPersistenceContext(state *persistence.State) PersistenceContext {
	return &persistenceContext{
		state: state,
	}
}

func (p *persistenceContext) GetSnapshot() persistence.Snapshot {
	return p.state.GetSnapshot()
}

func (p *persistenceContext) GetEvents() []persistence.Event {
	return p.state.GetEvents()
}

func (p *persistenceContext) Persist(event persistence.Event) {
	p.state.Update(event)
	// 立即保存到仓库
	p.state.Persist()
}

func (p *persistenceContext) SaveSnapshot(snapshot persistence.Snapshot) {
	p.state.SaveSnapshot(snapshot)
	// 立即保存到仓库
	p.state.Persist()
}

func (p *persistenceContext) CanRecover() bool {
	return p.state.GetSnapshot() != nil || len(p.state.GetEvents()) > 0
}

// PersistentActorFN 是一个函数类型，实现了 PersistentActor 接口（保持向后兼容）
type PersistentActorFN struct {
	OnReceiveFN     func(ctx ActorContext)
	OnRecoverFN     func(ctx PersistenceContext)
	PersistenceIdFN func() string
}

// OnReceive 实现 Actor 接口的 OnReceive 方法
func (fn PersistentActorFN) OnReceive(ctx ActorContext) {
	if fn.OnReceiveFN != nil {
		fn.OnReceiveFN(ctx)
	}
}

// OnRecover 实现 PersistentActor 接口的 OnRecover 方法
func (fn PersistentActorFN) OnRecover(ctx PersistenceContext) {
	if fn.OnRecoverFN != nil {
		fn.OnRecoverFN(ctx)
	}
}

// GetPersistenceId 实现 PersistentActor 接口的 GetPersistenceId 方法
func (fn PersistentActorFN) GetPersistenceId() string {
	if fn.PersistenceIdFN != nil {
		return fn.PersistenceIdFN()
	}
	return ""
}

// SmartPersistentActorFN 是增强版的函数类型，实现了 SmartPersistentActor 接口
type SmartPersistentActorFN struct {
	OnReceiveFN     func(ctx ActorContext)
	OnRecoverFN     func(ctx SmartPersistenceContext)
	PersistenceIdFN func() string
	GetStateFN      func() any
	ApplyEventFN    func(event persistence.Event)
}

// OnReceive 实现 Actor 接口的 OnReceive 方法
func (fn SmartPersistentActorFN) OnReceive(ctx ActorContext) {
	if fn.OnReceiveFN != nil {
		fn.OnReceiveFN(ctx)
	}
}

// OnRecover 实现 SmartPersistentActor 接口的 OnRecover 方法
func (fn SmartPersistentActorFN) OnRecover(ctx SmartPersistenceContext) {
	if fn.OnRecoverFN != nil {
		fn.OnRecoverFN(ctx)
	}
}

// GetPersistenceId 实现 SmartPersistentActor 接口的 GetPersistenceId 方法
func (fn SmartPersistentActorFN) GetPersistenceId() string {
	if fn.PersistenceIdFN != nil {
		return fn.PersistenceIdFN()
	}
	return ""
}

// GetCurrentState 实现 SmartPersistentActor 接口的 GetCurrentState 方法
func (fn SmartPersistentActorFN) GetCurrentState() any {
	if fn.GetStateFN != nil {
		return fn.GetStateFN()
	}
	return nil
}

// ApplyEvent 实现 SmartPersistentActor 接口的 ApplyEvent 方法
func (fn SmartPersistentActorFN) ApplyEvent(event persistence.Event) {
	if fn.ApplyEventFN != nil {
		fn.ApplyEventFN(event)
	}
}
