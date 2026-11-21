package gossip

import (
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/gossip/memberlist"
)

type dataCenterGroup struct {
	sequence   uint64
	sent       uint64
	priority   uint64
	round      uint64
	candidates []probeCandidate
	next       int
}

// dataCenterSchedule 在 Gossip Actor 线程内复用，空标识同时容纳未知和声明冲突的地址。
type dataCenterSchedule struct {
	addresses  map[string]string
	groups     map[string]*dataCenterGroup
	sequence   uint64
	candidates []probeCandidate
	dirty      bool
}

// selectDataCenterPeers 先为新观察轮换机房和地址，其余发送名额按发送历史轮换。
// 等待中的请求及普通 Gossip 不构成机房已获得新观察。
func (r *localReachability) selectDataCenterPeers(peers []vivid.ActorRef, members *memberlist.MemberList, sendLimit, probeLimit int) []vivid.ActorRef {
	limit, available := r.preparePeers(peers, sendLimit, probeLimit)
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
			if available == 0 {
				group.priority = group.sent
			}
		}
		if available == 0 {
			// 没有新观察名额时，直接收集普通发送候选，避免再次遍历所有地址。
			if limit > 0 {
				group.candidates = insertProbeCandidate(group.candidates, probeCandidate{index: index, sequence: r.attempts[address].sent}, limit)
			}
			continue
		}
		if _, waiting := r.pending[address]; waiting {
			continue
		}
		group.candidates = insertProbeCandidate(group.candidates, probeCandidate{index: index, sequence: r.attempts[address].sequence}, available)
	}
	for label, group := range schedule.groups {
		if group.round != r.scheduleRound {
			delete(schedule.groups, label)
		}
	}
	count, sequence := available, r.sequence
	candidates := r.candidates
	if available == 0 {
		count, sequence = limit, schedule.sequence
		candidates = schedule.candidates[:0]
	}
	for selected := range count {
		var oldest *dataCenterGroup
		for _, group := range schedule.groups {
			if group.next < len(group.candidates) && (oldest == nil || group.priority < oldest.priority) {
				oldest = group
			}
		}
		if oldest == nil {
			break
		}
		candidates = append(candidates, oldest.candidates[oldest.next])
		oldest.next++
		// 本轮暂排在已分配机房之后；持久顺序在实际创建请求或提交发送后更新。
		oldest.priority = sequence + uint64(selected) + 1
	}
	if available == 0 {
		schedule.candidates = candidates
		return prioritizeCandidates(peers, candidates, limit)
	}
	r.candidates = candidates
	r.prioritizePeers(peers, limit)
	if len(r.candidates) < limit {
		r.selectDataCenterSends(peers, len(r.candidates), limit)
	}
	return peers[:limit]
}

// selectDataCenterSends 为新观察以外的发送名额轮换机房和地址，不改变观察的先后顺序。
func (r *localReachability) selectDataCenterSends(peers []vivid.ActorRef, probes, limit int) {
	schedule := r.dataCenters
	remaining := limit - probes
	schedule.candidates = schedule.candidates[:0]
	for _, group := range schedule.groups {
		group.priority, group.next = group.sent, 0
		group.candidates = group.candidates[:0]
	}
	for index, peer := range peers[:probes] {
		// 已安排的新观察也会发送，普通名额先考虑其他尚未联络的机房。
		schedule.groups[schedule.addresses[peer.GetAddress()]].priority = schedule.sequence + uint64(index) + 1
	}
	for index, peer := range peers[probes:] {
		address := peer.GetAddress()
		group := schedule.groups[schedule.addresses[address]]
		group.candidates = insertProbeCandidate(group.candidates, probeCandidate{index: index, sequence: r.attempts[address].sent}, remaining)
	}
	for selected := range remaining {
		var oldest *dataCenterGroup
		for _, group := range schedule.groups {
			if group.next < len(group.candidates) && (oldest == nil || group.priority < oldest.priority) {
				oldest = group
			}
		}
		if oldest == nil {
			break
		}
		schedule.candidates = append(schedule.candidates, oldest.candidates[oldest.next])
		oldest.next++
		oldest.priority = schedule.sequence + uint64(probes+selected) + 1
	}
	prioritizeCandidates(peers[probes:], schedule.candidates, remaining)
}

// recordDataCenterSend 只记录交给消息系统的发送尝试，不表示连接成功或远端确认。
func (r *localReachability) recordDataCenterSend(peer vivid.ActorRef) {
	if r.dataCenters == nil {
		return
	}
	schedule := r.dataCenters
	schedule.sequence++
	if schedule.sequence == 0 {
		schedule.sequence++
	}
	address := peer.GetAddress()
	attempt := r.attempts[address]
	attempt.sent, attempt.round = schedule.sequence, r.scheduleRound
	r.attempts[address] = attempt
	schedule.groups[schedule.addresses[address]].sent = schedule.sequence
}

// updateAddresses 机房声明在实例内固定，仅在成员记录变化后重建地址分类。
func (s *dataCenterSchedule) updateAddresses(members *memberlist.MemberList) {
	clear(s.addresses)
	if members != nil {
		for _, member := range members.All() {
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
	}

	s.dirty = false
}

func (r *localReachability) invalidateDataCenters() {
	if r != nil && r.dataCenters != nil {
		r.dataCenters.dirty = true
	}
}
