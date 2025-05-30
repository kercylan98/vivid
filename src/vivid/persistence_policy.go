package vivid

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/kercylan98/vivid/src/persistence"
)

// SnapshotStrategy 定义快照策略接口
type SnapshotStrategy interface {
	// ShouldCreateSnapshot 判断是否应该创建快照
	ShouldCreateSnapshot(eventCount int, lastSnapshot time.Time, currentState any) bool
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

// ShouldCreateSnapshot 实现SnapshotStrategy接口
func (p *AutoSnapshotPolicy) ShouldCreateSnapshot(eventCount int, lastSnapshot time.Time, currentState any) bool {
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

// Serializer 序列化器接口，用于智能序列化状态
type Serializer interface {
	// Serialize 序列化对象
	Serialize(obj any) ([]byte, error)
	// Deserialize 反序列化对象
	Deserialize(data []byte, target any) error
	// DeepCopy 深拷贝对象
	DeepCopy(obj any) (any, error)
}

// JSONSerializer JSON序列化器实现
type JSONSerializer struct{}

func (j *JSONSerializer) Serialize(obj any) ([]byte, error) {
	return json.Marshal(obj)
}

func (j *JSONSerializer) Deserialize(data []byte, target any) error {
	return json.Unmarshal(data, target)
}

func (j *JSONSerializer) DeepCopy(obj any) (any, error) {
	if obj == nil {
		return nil, nil
	}

	// 使用JSON序列化实现深拷贝
	data, err := j.Serialize(obj)
	if err != nil {
		return nil, err
	}

	// 创建同类型的新对象
	objType := reflect.TypeOf(obj)
	if objType.Kind() == reflect.Ptr {
		// 如果是指针，创建新的实例
		newObj := reflect.New(objType.Elem()).Interface()
		err = j.Deserialize(data, newObj)
		return newObj, err
	} else {
		// 如果是值类型，直接反序列化
		newObj := reflect.New(objType).Interface()
		err = j.Deserialize(data, newObj)
		if err != nil {
			return nil, err
		}
		return reflect.ValueOf(newObj).Elem().Interface(), nil
	}
}

// SmartPersistenceManager 智能持久化管理器
type SmartPersistenceManager struct {
	state           *persistence.State
	policy          *AutoSnapshotPolicy
	serializer      Serializer
	eventCount      int
	lastSnapshot    time.Time
	currentSnapshot any
}

// NewSmartPersistenceManager 创建智能持久化管理器
func NewSmartPersistenceManager(state *persistence.State, policy *AutoSnapshotPolicy) *SmartPersistenceManager {
	if policy == nil {
		policy = DefaultSnapshotPolicy()
	}

	return &SmartPersistenceManager{
		state:        state,
		policy:       policy,
		serializer:   &JSONSerializer{},
		lastSnapshot: time.Now(),
	}
}

// PersistEvent 持久化事件（智能快照）
func (m *SmartPersistenceManager) PersistEvent(event persistence.Event, currentState any) error {
	// 持久化事件
	m.state.Update(event)
	m.eventCount++

	// 检查是否需要创建快照
	if m.policy.ShouldCreateSnapshot(m.eventCount, m.lastSnapshot, currentState) {
		if err := m.CreateSnapshot(currentState); err != nil {
			return fmt.Errorf("failed to create snapshot: %w", err)
		}
	}

	// 保存到仓库
	return m.state.Persist()
}

// CreateSnapshot 创建快照（自动深拷贝）
func (m *SmartPersistenceManager) CreateSnapshot(state any) error {
	// 自动深拷贝状态
	snapshot, err := m.serializer.DeepCopy(state)
	if err != nil {
		return fmt.Errorf("failed to deep copy state: %w", err)
	}

	// 保存快照
	m.state.SaveSnapshot(snapshot)
	m.currentSnapshot = snapshot
	m.eventCount = 0 // 重置事件计数
	m.lastSnapshot = time.Now()

	return m.state.Persist()
}

// ForceSnapshot 强制创建快照
func (m *SmartPersistenceManager) ForceSnapshot(state any) error {
	return m.CreateSnapshot(state)
}

// GetRecoveryData 获取恢复数据
func (m *SmartPersistenceManager) GetRecoveryData() (persistence.Snapshot, []persistence.Event, error) {
	err := m.state.Load()
	if err != nil {
		return nil, nil, err
	}
	return m.state.GetSnapshot(), m.state.GetEvents(), nil
}
