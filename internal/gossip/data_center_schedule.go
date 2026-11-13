package gossip

import (
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/gossip/endpoint"
)

type dataCenterGroup struct {
	sequence   uint64
	priority   uint64
	round      uint64
	candidates []probeCandidate
	next       int
}

// dataCenterSchedule 在 Gossip Actor 线程内复用，空标识同时容纳未知和声明冲突的地址。
type dataCenterSchedule struct {
	addresses map[string]string
	groups    map[string]*dataCenterGroup
	dirty     bool
}

// selectDataCenterPeers 先轮换有空闲候选的机房，再在机房内选择最久未开始观察的地址。
// 只为新观察分配机会，等待中的请求及普通 Gossip 不构成机房已获得新观察。
func (r *localReachability) selectDataCenterPeers(peers []vivid.ActorRef, members []*endpoint.Information, limit int) []vivid.ActorRef {
	limit, available := r.preparePeers(peers, limit)
	if r.dataCenters == nil {
		r.dataCenters = &dataCenterSchedule{addresses: make(map[string]string), groups: make(map[string]*dataCenterGroup), dirty: true}
	}
	schedule := r.dataCenters
	if schedule.dirty {
		schedule.updateAddresses(members)
	}
	for index, peer := range peers {
		address := peer.GetAddress()
		label := schedule.addresses[address]
		group := schedule.groups[label]
		if group == nil {
			group = &dataCenterGroup{}
			schedule.groups[label] = group
		}
		if group.round != r.scheduleRound {
			group.round, group.priority, group.next = r.scheduleRound, group.sequence, 0
			group.candidates = group.candidates[:0]
		}
		if _, waiting := r.pending[address]; waiting || available == 0 {
			continue
		}
		group.candidates = insertProbeCandidate(group.candidates, probeCandidate{index: index, sequence: r.attempts[address].sequence}, available)
	}
	for label, group := range schedule.groups {
		if group.round != r.scheduleRound {
			delete(schedule.groups, label)
		}
	}
	for selected := range available {
		var oldest *dataCenterGroup
		for _, group := range schedule.groups {
			if group.next < len(group.candidates) && (oldest == nil || group.priority < oldest.priority) {
				oldest = group
			}
		}
		if oldest == nil {
			break
		}
		r.candidates = append(r.candidates, oldest.candidates[oldest.next])
		oldest.next++
		// 本轮暂排在已分配机房之后；持久顺序仅在 begin 实际创建请求时更新。
		oldest.priority = r.sequence + uint64(selected) + 1
	}
	return r.prioritizePeers(peers, limit)
}

// updateAddresses 机房声明在实例内固定，仅在成员记录变化后重建地址分类。
func (s *dataCenterSchedule) updateAddresses(members []*endpoint.Information) {
	clear(s.addresses)
	for _, member := range members {
		if !observableMember(member.Status) {
			continue
		}
		address := member.ActorRef.GetAddress()
		label, exists := s.addresses[address]
		if !exists {
			s.addresses[address] = member.DataCenter
		} else if label != member.DataCenter {
			// 不按实例 ID、创建时间或插入顺序猜测同地址当前属于哪个机房。
			s.addresses[address] = ""
		}
	}
	s.dirty = false
}

func (r *localReachability) invalidateDataCenters() {
	if r != nil && r.dataCenters != nil {
		r.dataCenters.dirty = true
	}
}
