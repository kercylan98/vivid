// Package memberlist 维护集群成员列表（节点 ID -> Information），支持 Add/Upsert/Merge 与序列化。
package memberlist

import (
	"sort"
	"strings"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/internal/serialization"
	"github.com/kercylan98/vivid/pkg/log"
)

var (
	_ serialization.MessageCodec = (*MemberList)(nil)
)

// New 创建空成员列表，logger 用于成员变更的调试日志。
func New(logger log.Logger) *MemberList {
	return &MemberList{
		logger: logger,
	}
}

// MemberList 集群成员列表：key 为节点 ID（endpoint.Information.ID()），value 为端点信息。
// 实现 serialization.MessageCodec，可与 VersionVector 一起随 Ping/Pong 序列化传输。
type MemberList struct {
	logger  log.Logger              // 用于成员变更的调试日志。
	members []*endpoint.Information // 成员列表
}

// GetCoordinatorNodeID 获取当前的协调者节点 ID。
func (m *MemberList) GetCoordinatorNodeID() string {
	if len(m.members) == 0 {
		return ""
	}

	// 按创建时间升序，如果创建时间相同，则按 ActorRef 字典序升序
	sort.Slice(m.members, func(i, j int) bool {
		aCreatedAt := m.members[i].CreatedAt.UnixNano()
		jCreatedAt := m.members[j].CreatedAt.UnixNano()
		if aCreatedAt == jCreatedAt {
			return m.members[i].ActorRef.String() < m.members[j].ActorRef.String()
		}
		return aCreatedAt < jCreatedAt
	})

	return m.members[0].ID()
}

// Decode 从 reader 反序列化成员列表，实现 MessageCodec。
func (m *MemberList) Decode(reader *serialization.Reader, message any) error {
	msg := message.(*MemberList)
	return reader.Read(&msg.members)
}

// Encode 将成员列表序列化到 writer，实现 MessageCodec。
func (m *MemberList) Encode(writer *serialization.Writer, message any) error {
	msg := message.(*MemberList)
	return writer.Write(msg.members).Err()
}

// Get 获取指定节点信息。
func (m *MemberList) Get(id string) *endpoint.Information {
	for _, member := range m.members {
		if member.ID() == id {
			return member
		}
	}
	return nil
}

// Upsert 添加或覆盖指定节点信息（同一 ID 直接覆盖），用于本节点状态写回与 gossip 传播后的更新。
//
// 如果节点信息不存在，则返回 true，否则返回 false。
func (m *MemberList) Upsert(info *endpoint.Information) (isNew bool) {
	var (
		id       = info.ID()
		oldIndex int
		old      *endpoint.Information
	)

	isNew = true
	for oldIndex = 0; oldIndex < len(m.members); oldIndex++ {
		member := m.members[oldIndex]
		if member.ID() == id {
			old = member
			isNew = false
			break
		}
	}
	if !isNew {
		if old.Status != info.Status {
			m.logger.Debug("member status changed", log.String("ref", info.ActorRef.String()), log.String("before", old.Status.String()), log.String("current", info.Status.String()))
		}
		m.members[oldIndex] = info
		return isNew
	}
	m.members = append(m.members, info)
	return isNew
}

// Unseens 从列表中选取最多 limit 个 StatusUp 或 StatusLeaving 的节点（排除 local 自身），用于本轮 gossip 的 peer 选择。
func (m *MemberList) Unseens(local *endpoint.Information, limit int) []*endpoint.Information {
	if limit <= 0 || len(m.members) == 0 {
		return nil
	}

	out := make([]*endpoint.Information, 0, limit)
	for _, member := range m.members {
		if member.ActorRef.Equals(local.ActorRef) {
			continue
		}

		switch member.Status {
		case endpoint.StatusJoining, endpoint.StatusRemoved:
		default:
			out = append(out, member)
		}

		if len(out) >= limit {
			break
		}
	}
	return out
}

// Merge 将 other 的成员合并到本列表（同 ID 以 other 为准）；调用方应保证仅在本地版本向量早于对方时调用，以保持因果一致。
func (m *MemberList) Merge(other *MemberList) {
	if other == nil {
		return
	}

	for _, member := range other.List() {
		localMember := m.Get(member.ID())
		if localMember != nil {
			if localMember.Status != member.Status {
				m.logger.Debug("member status changed", log.String("ref", member.ActorRef.String()), log.String("before", localMember.Status.String()), log.String("current", member.Status.String()))
			}
			localMember.Status = member.Status
		} else {
			m.logger.Debug("member added", log.String("ref", member.ActorRef.String()), log.String("status", member.Status.String()))
			m.members = append(m.members, member)
		}
	}
}

// Fingerprint 返回确定性的成员列表指纹（按节点 ID 排序，每项为 id+status），用于收敛检测。
func (m *MemberList) Fingerprint() string {
	if m == nil || len(m.members) == 0 {
		return ""
	}

	var fingerprints = make([]string, len(m.members))
	for i, member := range m.members {
		fingerprints[i] = member.ID() + ":" + member.Status.String()
	}
	sort.Strings(fingerprints)
	return strings.Join(fingerprints, ",")
}

// Remove 从成员列表中移除指定节点。
func (m *MemberList) Remove(ref vivid.ActorRef) {
	for i, member := range m.members {
		if member.ActorRef.Equals(ref) {
			m.members = append(m.members[:i], m.members[i+1:]...)
			break
		}
	}
}

// List 返回成员列表。
func (m *MemberList) List() []*endpoint.Information {
	return m.members
}
