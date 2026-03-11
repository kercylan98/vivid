// Package memberlist 维护集群成员列表（节点 ID -> Information），支持 Add/Upsert/Merge 与序列化。
package memberlist

import (
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

// Add 将节点加入列表；若该节点已存在则返回 ErrorGossipMemberAlreadyExists，用于保证新成员唯一加入。
func (m *MemberList) Add(info *endpoint.Information) error {
	if info == nil {
		return vivid.ErrorGossipInvalidMember.WithMessage("info is nil")
	}

	if _, ok := m.members[info.ActorRef.String()]; ok {
		return vivid.ErrorGossipMemberAlreadyExists.WithMessage(info.ActorRef.String())
	}

	m.members[info.ActorRef.String()] = info
	return nil
}

// Upsert 添加或覆盖指定节点信息（同一 ID 直接覆盖），用于本节点状态写回与 gossip 传播后的更新。
func (m *MemberList) Upsert(info *endpoint.Information) {
	if info == nil || m.members == nil {
		return
	}
	key := info.ActorRef.String()
	if local, ok := m.members[key]; !ok {
		m.logger.Debug("upsert member", log.String("ref", key), log.String("status", info.Status.String()))
	} else {
		if local.Status != info.Status {
			m.logger.Debug("member status changed", log.String("ref", key), log.String("before", local.Status.String()), log.String("current", info.Status.String()))
		}
	}
	m.members[key] = info
}

// Unseens 从列表中选取最多 limit 个 StatusUp 的节点（排除 local 自身），用于本轮 gossip 的 peer 选择。
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
		case endpoint.StatusUp:
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
		beforeStatus := endpoint.StatusNone
		if localMember, ok := m.members[member.ActorRef.String()]; ok {
			beforeStatus = localMember.Status
		}

		m.logger.Debug("member status changed", log.String("ref", member.ActorRef.String()), log.String("before", beforeStatus.String()), log.String("current", member.Status.String()))

		m.members[member.ActorRef.String()] = member
	}
}
