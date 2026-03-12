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
		logger:  logger,
		members: make(map[string]*endpoint.Information),
	}
}

// MemberList 集群成员列表：key 为节点 ID（endpoint.Information.ID()），value 为端点信息。
// 实现 serialization.MessageCodec，可与 VersionVector 一起随 Ping/Pong 序列化传输。
type MemberList struct {
	logger  log.Logger
	members map[string]*endpoint.Information
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

// Upsert 添加或覆盖指定节点信息（同一 ID 直接覆盖），用于本节点状态写回与 gossip 传播后的更新。
//
// 如果节点信息不存在，则返回 true，否则返回 false。
func (m *MemberList) Upsert(info *endpoint.Information) (isNew bool) {
	key := info.ActorRef.String()
	old, exists := m.members[key]
	if exists && old.Status != info.Status {
		m.logger.Debug("member status changed", log.String("ref", info.ActorRef.String()), log.String("before", old.Status.String()), log.String("current", info.Status.String()))
	}
	m.members[key] = info
	return !exists
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

	if m.members == nil {
		m.members = make(map[string]*endpoint.Information, len(other.members))
	}

	for _, member := range other.members {
		localMember, ok := m.members[member.ActorRef.String()]
		if ok {
			if localMember.Status != member.Status {
				m.logger.Debug("member status changed", log.String("ref", member.ActorRef.String()), log.String("before", localMember.Status.String()), log.String("current", member.Status.String()))
			}
		} else {
			m.logger.Debug("member added", log.String("ref", member.ActorRef.String()), log.String("status", member.Status.String()))
		}

		m.members[member.ActorRef.String()] = member
	}
}

// Fingerprint 返回确定性的成员列表指纹（按节点 ID 排序，每项为 id+status），用于收敛检测。
func (m *MemberList) Fingerprint() string {
	if m == nil || len(m.members) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m.members))
	for k := range m.members {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(k)
		b.WriteByte(':')
		b.WriteString(m.members[k].Status.String())
	}
	return b.String()
}

// Remove 从成员列表中移除指定节点。
func (m *MemberList) Remove(ref vivid.ActorRef) {
	delete(m.members, ref.String())
}

// List 返回成员列表。
func (m *MemberList) List() []*endpoint.Information {
	list := make([]*endpoint.Information, 0, len(m.members))
	for _, member := range m.members {
		list = append(list, member)
	}
	return list
}
