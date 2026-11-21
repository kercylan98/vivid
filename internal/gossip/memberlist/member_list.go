// Package memberlist 维护按实例 ID 区分的集群成员列表及其序列化。
package memberlist

import (
	"iter"
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

// New 复制给定的记录值并保留顺序，logger 用于成员变更的调试日志。
func New(logger log.Logger, members ...endpoint.Information) *MemberList {
	return &MemberList{
		logger:  logger,
		members: append([]endpoint.Information(nil), members...),
	}
}

// MemberList 按首次接收顺序保存成员记录，写入时按实例 ID 建立索引。
// 实现 serialization.MessageCodec，可与 VersionVector 一起随 Ping/Pong 序列化传输。
type MemberList struct {
	logger  log.Logger             // 用于成员变更的调试日志。
	members []endpoint.Information // 按首次接收顺序保留的独立成员记录
	indexes map[string]int         // 写入时建立，克隆与解码的只读视图不保留索引
}

// Clone 深拷贝当前成员列表。
func (m *MemberList) Clone() *MemberList {
	var clone = &MemberList{
		logger:  m.logger,
		members: make([]endpoint.Information, len(m.members)),
	}
	for i, member := range m.members {
		clone.members[i] = member
		clone.members[i].ActorRef = member.ActorRef.Clone()
	}
	return clone
}

// GetCoordinatorNodeID 获取当前的协调者节点 ID
func (m *MemberList) GetCoordinatorNodeID() string {
	// 首先筛选状态为UP的节点
	var upMembers []*endpoint.Information
	for index := range m.members {
		member := &m.members[index]
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
	// 先解码到独立存储，失败时保留原记录和索引。
	var members []endpoint.Information
	if err := reader.Read(&members); err != nil {
		return err
	}
	msg.members, msg.indexes = members, nil
	return nil
}

// Encode 将成员列表序列化到 writer，实现 MessageCodec。
func (m *MemberList) Encode(writer *serialization.Writer, message any) error {
	msg := message.(*MemberList)
	return writer.Write(msg.members).Err()
}

// Get 返回指定实例的独立记录，修改结果需通过 Upsert 写回。
// 只读视图不在查询时建立索引，可与异步编码并行读取。
func (m *MemberList) Get(id string) *endpoint.Information {
	if m.indexes != nil {
		if index, exists := m.indexes[id]; exists {
			member := m.members[index]
			return &member
		}
		return nil
	}
	for _, member := range m.members {
		if member.ID() == id {
			return &member
		}
	}
	return nil
}

// Upsert 复制并写入成员记录；覆盖保持原位置，新实例追加到末尾。
// 返回是否新增。实例 ID 是索引键，修改调用方记录不会改变已保存的身份。
func (m *MemberList) Upsert(info *endpoint.Information) bool {
	if info == nil {
		return false
	}
	if m.indexes == nil {
		m.indexes = make(map[string]int, len(m.members)+1)
		for index, member := range m.members {
			// 解码保留原始列表，重复实例沿用首条查找与覆盖语义。
			if _, exists := m.indexes[member.ID()]; !exists {
				m.indexes[member.ID()] = index
			}
		}
	}
	id := info.ID()
	if index, exists := m.indexes[id]; exists {
		old := m.members[index]
		if old.Status != info.Status && m.logger != nil {
			m.logger.Debug("member status changed", log.String("ref", info.ActorRef.String()), log.String("before", old.Status.String()), log.String("current", info.Status.String()))
		}
		m.members[index] = *info
		return false
	}
	m.indexes[id] = len(m.members)
	m.members = append(m.members, *info)
	return true
}

// Unseens 按通信地址去重并随机选取最多 limit 个目标；Up 节点持续联络配置的种子。
func (m *MemberList) Unseens(local *endpoint.Information, seeds []vivid.ActorRef, limit int) []vivid.ActorRef {
	if limit <= 0 {
		return nil
	}
	out := make([]vivid.ActorRef, 0, len(m.members)+len(seeds))
	seen := make(map[string]struct{}, len(m.members)+len(seeds))
	for _, member := range m.members {
		if member.ActorRef.GetAddress() == local.ActorRef.GetAddress() {
			continue
		}
		switch member.Status {
		case endpoint.StatusUp, endpoint.StatusLeaving, endpoint.StatusExiting:
			address := member.ActorRef.GetAddress()
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
			if seed == nil || seed.GetAddress() == local.ActorRef.GetAddress() {
				continue
			}
			id := seed.GetAddress()
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

// Len 返回保留的成员记录数。
func (m *MemberList) Len() int { return len(m.members) }

// At 返回指定位置的独立记录值，越界时与切片索引一样 panic。
func (m *MemberList) At(index int) endpoint.Information { return m.members[index] }

// All 按保存顺序遍历记录值，不分配列表；迭代期间不能调用 Upsert 或 Decode。
// ActorRef 沿用不可变引用，修改记录字段不影响成员列表。
func (m *MemberList) All() iter.Seq2[int, endpoint.Information] {
	return func(yield func(int, endpoint.Information) bool) {
		for index, member := range m.members {
			if !yield(index, member) {
				return
			}
		}
	}
}

// List 返回独立的记录列表，ActorRef 沿用不可变引用；热路径遍历使用 All。
func (m *MemberList) List() []*endpoint.Information {
	members := append([]endpoint.Information(nil), m.members...)
	list := make([]*endpoint.Information, len(members))
	for index := range members {
		list[index] = &members[index]
	}
	return list
}
