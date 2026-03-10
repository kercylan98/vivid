package memberlist

import (
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/gossip/internal/endpoint"
	"github.com/kercylan98/vivid/internal/messages"
)

func init() {
	messages.RegisterInternalMessage[*MemberList]("MemberList", onMemberListReader, onMemberListWriter)
}

func New() *MemberList {
	return &MemberList{
		members: make(map[string]*endpoint.Information),
	}
}

// MemberList 成员列表
type MemberList struct {
	members map[string]*endpoint.Information // 成员列表，key 为节点 ActorRef 字符串，value 为节点信息
}

func (m *MemberList) Add(info *endpoint.Information) error {
	if info == nil {
		return vivid.ErrorGossipInvalidMember.WithMessage("info is nil")
	}

	if _, ok := m.members[info.ActorRef.String()]; ok {
		return vivid.ErrorGossipMemberAlreadyExists.WithMessage(info.ActorRef.String())
	}

	info.SetStatus(endpoint.StatusUp)
	m.members[info.ActorRef.String()] = info
	return nil
}

func onMemberListReader(message any, reader *messages.Reader, codec messages.Codec) error {
	m := message.(*MemberList)
	if m.members == nil {
		m.members = make(map[string]*endpoint.Information)
	}
	return reader.Read(&m.members)
}

func onMemberListWriter(message any, writer *messages.Writer, codec messages.Codec) error {
	m := message.(*MemberList)
	writer.Write(m.members)
	return writer.Err()
}

// Unseens 返回用于本轮 gossip 的节点列表，最多返回 limit 个节点。
// 当前实现按存储顺序返回前 N 个节点，后续可以替换为随机策略。
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

// Merge 将 other 的成员合并到本地。调用方必须在「本地版本向量严格早于对方」时才调用，
// 此时对方视图在因果上更新，因此对同一 ID 直接采纳对方条目，不做逐条新旧比较。
func (m *MemberList) Merge(other *MemberList) {
	if other == nil {
		return
	}

	if m.members == nil {
		m.members = make(map[string]*endpoint.Information, len(other.members))
	}

	for _, member := range other.members {
		if member == nil {
			continue
		}
		m.members[member.ActorRef.String()] = member
	}
}
