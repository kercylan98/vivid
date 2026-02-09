package cluster

import (
	"time"

	"github.com/google/uuid"
)

// ClusterProtocolVersion 集群协议版本号，随 ClusterView 序列化，用于跨版本滚动升级与兼容性校验。
const ClusterProtocolVersion uint16 = 1

func newClusterView() *ClusterView {
	return &ClusterView{
		ViewID:         uuid.New().String(),
		Epoch:          0,
		Timestamp:      time.Now().UnixNano(),
		Members:        make(map[string]*NodeState),
		HealthyCount:   0,
		UnhealthyCount: 0,
		ProtocolVersion: ClusterProtocolVersion,
	}
}

// ClusterView 表示当前节点持有的集群成员视图，用于 Gossip 与故障检测。
// Epoch 与 VersionVector 用于合并时的一致性判断与脑裂防护。
// ProtocolVersion 用于序列化/反序列化与跨版本兼容，小版本只增字段不删以保证向后兼容。
type ClusterView struct {
	ViewID                   string
	Epoch                    int64
	Timestamp                int64
	Members                  map[string]*NodeState
	HealthyCount             int
	UnhealthyCount           int
	QuorumSize               int
	VersionVector            VersionVector
	ProtocolVersion          uint16
	MaxVersionVectorEntries  int // 0 表示使用默认 65535
}

// AddMember 将成员加入视图；若已存在则仅在入参为更新分代/时间戳时覆盖（避免脑裂采纳旧实例）。
// 内部存储克隆，避免调用方修改影响视图一致性。
func (v *ClusterView) AddMember(member *NodeState) {
	if member == nil {
		return
	}
	if v.Members == nil {
		v.Members = make(map[string]*NodeState)
	}
	existing, ok := v.Members[member.ID]
	if ok && !member.IsNewerThan(existing) {
		return
	}
	v.Members[member.ID] = member.Clone()
	v.recomputeCounts()
}

// IncrementVersion 在本地做出成员变更后调用，递增指定节点的版本向量分量，用于因果顺序与合并判断。
// localNodeID 为做出变更的节点 ID（通常为本节点）。
func (v *ClusterView) IncrementVersion(localNodeID string) {
	if localNodeID == "" {
		return
	}
	next, err := v.VersionVector.Increment(localNodeID)
	if err != nil {
		return
	}
	v.VersionVector = next
}

func (v *ClusterView) MemberByAddress(address string) *NodeState {
	for _, m := range v.Members {
		if m.Address == address {
			return m
		}
	}
	return nil
}

func (v *ClusterView) RemoveMember(nodeID string) {
	if _, ok := v.Members[nodeID]; ok {
		delete(v.Members, nodeID)
		v.recomputeCounts()
	}
}

// Snapshot 返回视图的不可变快照（深拷贝成员状态），用于 Gossip 与响应，避免调用方修改影响本视图。
func (v *ClusterView) Snapshot() *ClusterView {
	if v == nil {
		return nil
	}
	members := make(map[string]*NodeState, len(v.Members))
	for id, state := range v.Members {
		if state != nil {
			members[id] = state.Clone()
		}
	}
	return &ClusterView{
		ViewID:                  v.ViewID,
		Epoch:                   v.Epoch,
		Timestamp:               v.Timestamp,
		Members:                 members,
		HealthyCount:            v.HealthyCount,
		UnhealthyCount:          v.UnhealthyCount,
		QuorumSize:              v.QuorumSize,
		VersionVector:           v.VersionVector.Clone(),
		ProtocolVersion:         v.ProtocolVersion,
		MaxVersionVectorEntries: v.MaxVersionVectorEntries,
	}
}

// MergeOptions 合并视图时的可选策略，用于时钟偏差与 VersionConcurrent 场景。
type MergeOptions struct {
	MaxClockSkew              time.Duration // 若 >0 且 other.Timestamp 与本地差超过此值，不采纳 other 的 Epoch/Timestamp
	VersionConcurrentStrategy int           // 0=TakeMax, 1=PreferLocal, 2=PreferRemote
}

// MergeFrom 将 other 的成员与版本信息合并到本视图，使用默认策略（TakeMax、不校验时钟偏差）。
func (v *ClusterView) MergeFrom(other *ClusterView) {
	v.MergeFromWithOptions(other, MergeOptions{})
}

// MergeFromWithOptions 将 other 合并到本视图，并按 opts 应用时钟偏差与 VersionConcurrent 策略。
// 返回 true 表示本次合并导致视图发生变更（成员或版本向量等），调用方可用于决定是否需要继续扩散 Gossip。
// 合并规则：同一节点按 IsNewerThan 采纳；版本向量取并合并。
// 当 VersionConcurrent 时按 VersionConcurrentStrategy 决定是否采纳 other 的 Epoch/Timestamp；
// 当 MaxClockSkew>0 且 other.Timestamp 与本地差超过阈值时不采纳 other 的 Epoch/Timestamp。
func (v *ClusterView) MergeFromWithOptions(other *ClusterView, opts MergeOptions) (changed bool) {
	if other == nil || other.Members == nil || len(other.Members) == 0 {
		return false
	}
	if v.Members == nil {
		v.Members = make(map[string]*NodeState)
	}
	concurrent := v.VersionVector.Compare(other.VersionVector) == VersionConcurrent
	for id, otherState := range other.Members {
		if otherState == nil {
			continue
		}
		existing, ok := v.Members[id]
		if !ok || otherState.IsNewerThan(existing) {
			v.Members[id] = otherState.Clone()
			changed = true
		}
	}
	v.recomputeCounts()
	mergedVV := v.VersionVector.Merge(other.VersionVector)
	if !mergedVV.Equal(v.VersionVector) {
		changed = true
	}
	v.VersionVector = mergedVV

	skipEpochTimestamp := false
	if opts.MaxClockSkew > 0 {
		nowNano := time.Now().UnixNano()
		diff := nowNano - other.Timestamp
		if diff < 0 {
			diff = -diff
		}
		if diff > opts.MaxClockSkew.Nanoseconds() {
			skipEpochTimestamp = true
		}
	}
	if !skipEpochTimestamp {
		adoptEpochTimestamp := true
		if concurrent {
			switch opts.VersionConcurrentStrategy {
			case 1: // PreferLocal
				adoptEpochTimestamp = false
			case 2: // PreferRemote
				adoptEpochTimestamp = true
			default: // TakeMax
				adoptEpochTimestamp = true
			}
		}
		if adoptEpochTimestamp {
			if other.Epoch > v.Epoch {
				v.Epoch = other.Epoch
				changed = true
			}
			if other.Timestamp > v.Timestamp {
				v.Timestamp = other.Timestamp
				changed = true
			}
		}
	}
	if other.ProtocolVersion > v.ProtocolVersion {
		v.ProtocolVersion = other.ProtocolVersion
		changed = true
	}
	return changed
}

func (v *ClusterView) recomputeCounts() {
	v.HealthyCount = 0
	v.UnhealthyCount = 0
	activeNodes := make([]string, 0, len(v.Members))
	for id, m := range v.Members {
		if m == nil {
			continue
		}
		activeNodes = append(activeNodes, id)
		// Suspect 不计入健康，避免分区时两边都自认 quorum
		if m.Status == MemberStatusUp {
			v.HealthyCount++
		} else {
			v.UnhealthyCount++
		}
	}
	if v.HealthyCount > 0 {
		v.QuorumSize = (v.HealthyCount / 2) + 1
	} else {
		v.QuorumSize = 0
	}
	// 状态压缩：版本向量只保留当前成员，防止无限增长
	if len(activeNodes) > 0 {
		maxEnt := v.MaxVersionVectorEntries
		v.VersionVector = v.VersionVector.PruneWithMax(activeNodes, maxEnt)
	}
}
