// Package memberlist 维护按实例 ID 区分的集群成员列表及其序列化。
package memberlist

import (
	"math/rand/v2"
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

// MemberList 集群成员列表：key 为实例 ID（endpoint.Information.ID()），value 为端点信息。
// 实现 serialization.MessageCodec，可与 VersionVector 一起随 Ping/Pong 序列化传输。
type MemberList struct {
	logger  log.Logger              // 用于成员变更的调试日志。
	members []*endpoint.Information // 成员列表
}

// Clone 深拷贝当前成员列表。
func (m *MemberList) Clone() *MemberList {
	var clone = &MemberList{
		logger:  m.logger,
		members: make([]*endpoint.Information, len(m.members)),
	}
	for i, member := range m.members {
		clone.members[i] = member.Clone()
	}
	return clone
}

// GetCoordinatorNodeID 获取当前的协调者节点 ID
func (m *MemberList) GetCoordinatorNodeID() string {
	// 首先筛选状态为UP的节点
	var upMembers []*endpoint.Information
	for _, member := range m.members {
		if member.Status == endpoint.StatusUp {
			upMembers = append(upMembers, member)
		}
	}

	if len(upMembers) == 0 {
		// 如果没有UP状态的节点，返回空
		return ""
	}

	// 按创建时间升序，同时间按通信地址和实例 ID 排序；此标识仅用于协调者日志
	sort.Slice(upMembers, func(i, j int) bool {
		aCreatedAt := upMembers[i].CreatedAt.UnixNano()
		jCreatedAt := upMembers[j].CreatedAt.UnixNano()
		if aCreatedAt == jCreatedAt {
			aRef, jRef := upMembers[i].ActorRef.String(), upMembers[j].ActorRef.String()
			if aRef == jRef {
				return upMembers[i].ID() < upMembers[j].ID()
			}
			return aRef < jRef
		}
		return aCreatedAt < jCreatedAt
	})

	return upMembers[0].ID()
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

// Unseens 按通信地址去重并随机选取最多 limit 个目标；Up 节点持续联络配置的种子。
func (m *MemberList) Unseens(local *endpoint.Information, seeds []vivid.ActorRef, limit int) []vivid.ActorRef {
	if limit <= 0 {
		return nil
	}
	out := make([]vivid.ActorRef, 0, len(m.members)+len(seeds))
	seen := make(map[string]struct{}, len(m.members)+len(seeds))
	for _, member := range m.members {
		if member.ActorRef.Equals(local.ActorRef) {
			continue
		}
		switch member.Status {
		case endpoint.StatusUp, endpoint.StatusLeaving, endpoint.StatusExiting:
			address := member.ActorRef.String()
			if _, exists := seen[address]; exists {
				continue
			}
			seen[address] = struct{}{}
			out = append(out, member.ActorRef)
		}
	}
	// 种子是联络地址，旧实例的退出墓碑不能阻止同地址新实例的发现；退出期间不再探测种子。
	if local.Status == endpoint.StatusUp {
		for _, seed := range seeds {
			if seed == nil || seed.Equals(local.ActorRef) {
				continue
			}
			id := seed.String()
			if _, exists := seen[id]; exists {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, seed)
		}
	}
	// 成员与未知种子一起抽样，避免少量固定目标占满每轮额度。
	rand.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	if len(out) > limit {
		return out[:limit]
	}
	return out
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

// List 返回成员列表。
func (m *MemberList) List() []*endpoint.Information {
	return m.members
}
