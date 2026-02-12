package cluster

import "time"

// 多数据中心 / 拓扑标签键，用于 NodeState.Labels，供 Gossip 与故障检测区分同 DC / 跨 DC；Region/Zone 用于全球多层级拓扑。
const (
	LabelDatacenter = "datacenter"
	LabelRack       = "rack"
	LabelRegion     = "region"
	LabelZone       = "zone"
)

type MemberStatus int

const (
	MemberStatusJoining     MemberStatus = iota
	MemberStatusUp
	MemberStatusSuspect
	MemberStatusUnreachable
	MemberStatusDown
	MemberStatusLeaving
	MemberStatusExiting
	MemberStatusRemoved
)

func (s MemberStatus) String() string {
	switch s {
	case MemberStatusJoining:
		return "joining"
	case MemberStatusUp:
		return "up"
	case MemberStatusSuspect:
		return "suspect"
	case MemberStatusUnreachable:
		return "unreachable"
	case MemberStatusDown:
		return "down"
	case MemberStatusLeaving:
		return "leaving"
	case MemberStatusExiting:
		return "exiting"
	case MemberStatusRemoved:
		return "removed"
	default:
		return "unknown"
	}
}

func newNodeState(id string, clusterName string, address string) *NodeState {
	now := time.Now().UnixNano()
	return &NodeState{
		ID:           id,
		ClusterName:  clusterName,
		Address:      address,
		Generation:   1,
		Timestamp:    now,
		SeqNo:        0,
		Status:       MemberStatusJoining,
		Unreachable:  false,
		LastSeen:     now,
		LogicalClock: 1,
		Metadata:     make(map[string]string),
		Labels:       make(map[string]string),
		CustomState:  make(map[string]string),
		Checksum:     0,
	}
}

// Datacenter 返回节点所在数据中心标识（来自 Labels[LabelDatacenter]），空表示未配置。
func (n *NodeState) Datacenter() string {
	if n == nil || n.Labels == nil {
		return ""
	}
	return n.Labels[LabelDatacenter]
}

// Rack 返回节点所在机架标识（来自 Labels[LabelRack]），空表示未配置。
func (n *NodeState) Rack() string {
	if n == nil || n.Labels == nil {
		return ""
	}
	return n.Labels[LabelRack]
}

// Region 返回节点所在区域标识（来自 Labels[LabelRegion]），用于同 Region 优先 Gossip。
func (n *NodeState) Region() string {
	if n == nil || n.Labels == nil {
		return ""
	}
	return n.Labels[LabelRegion]
}

// Zone 返回节点所在可用区标识（来自 Labels[LabelZone]）。
func (n *NodeState) Zone() string {
	if n == nil || n.Labels == nil {
		return ""
	}
	return n.Labels[LabelZone]
}

// NodeState 表示集群中某一节点的状态，用于 Gossip 与故障检测。
// 节点在视图中的因果版本由 ClusterView.VersionVector 维护，GetMembers 等从 VersionVector.Get(nodeID) 获取。
// Generation 在节点重启后递增，用于区分同一节点的不同 incarnation，避免脑裂时采纳旧实例。
// Labels 可携带 datacenter/rack 等拓扑信息，用于多数据中心与跨 DC 故障检测。
// CustomState 为运行时可变的自定义状态，通过 ClusterContext.UpdateNodeState 更新并随 Gossip 传播；Metadata/Labels 适合固定常量。
// LogicalClock 本节点产生的状态变更的逻辑时钟，用于同一节点多实例的偏序比较，减少对物理时钟偏差的依赖；0 表示未使用。
type NodeState struct {
	ID           string
	ClusterName  string
	Address      string
	Generation   int    // 重启分代，每次进程重启递增，用于脑裂与重启检测
	Timestamp    int64
	SeqNo        uint64
	Status       MemberStatus
	Unreachable  bool
	LastSeen     int64
	LogicalClock uint64 // 本节点逻辑时钟，同一节点比较时优先于 Timestamp
	Metadata     map[string]string // 固定/常量元数据，通常加入时确定
	Labels       map[string]string // 固定/常量标签（如 datacenter/rack），用于拓扑与故障检测
	CustomState  map[string]string // 运行时可变的自定义状态，可随时更新并随 Gossip 传播
	Checksum     uint32
}

// Clone 深拷贝 NodeState，避免视图间共享可变引用。
func (n *NodeState) Clone() *NodeState {
	if n == nil {
		return nil
	}
	out := *n
	if len(n.Metadata) > 0 {
		out.Metadata = make(map[string]string, len(n.Metadata))
		for k, v := range n.Metadata {
			out.Metadata[k] = v
		}
	}
	if len(n.Labels) > 0 {
		out.Labels = make(map[string]string, len(n.Labels))
		for k, v := range n.Labels {
			out.Labels[k] = v
		}
	}
	if len(n.CustomState) > 0 {
		out.CustomState = make(map[string]string, len(n.CustomState))
		for k, v := range n.CustomState {
			out.CustomState[k] = v
		}
	}
	return &out
}

// IsNewerThan 按分代、逻辑时钟与时间戳判断是否比 other 更新（用于合并时采纳新状态、避免脑裂采纳旧实例）。
// 先比较 Generation；同分代且为同一节点时若双方均有 LogicalClock 则比较 LogicalClock；否则比较 Timestamp。
func (n *NodeState) IsNewerThan(other *NodeState) bool {
	if other == nil {
		return true
	}
	if n.Generation != other.Generation {
		return n.Generation > other.Generation
	}
	if n.ID == other.ID && n.LogicalClock != 0 && other.LogicalClock != 0 {
		return n.LogicalClock > other.LogicalClock
	}
	return n.Timestamp > other.Timestamp
}
