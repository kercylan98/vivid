package gossip

import "github.com/kercylan98/vivid"

type probeAttempt struct {
	sequence uint64
	round    uint64
}

type probeCandidate struct {
	index    int
	sequence uint64
}

// selectPeers 将空闲观察额度交给最久未开始探测的地址，其余名额保留随机 Gossip。
// peers 已按地址去重并打乱；只调整前 limit 项，不需要排序整个成员列表。
func (r *localReachability) selectPeers(peers []vivid.ActorRef, limit int) []vivid.ActorRef {
	limit, available := r.preparePeers(peers, limit)
	if available == 0 {
		return peers[:limit]
	}
	for index, peer := range peers {
		address := peer.GetAddress()
		if _, waiting := r.pending[address]; waiting {
			continue
		}
		r.candidates = insertProbeCandidate(r.candidates, probeCandidate{index: index, sequence: r.attempts[address].sequence}, available)
	}
	return r.prioritizePeers(peers, limit)
}

// preparePeers 清理已经退出候选集合的地址，并计算本轮可开始的观察数量。
func (r *localReachability) preparePeers(peers []vivid.ActorRef, limit int) (int, int) {
	r.scheduleRound++
	for _, peer := range peers {
		address := peer.GetAddress()
		if attempt, exists := r.attempts[address]; exists {
			attempt.round = r.scheduleRound
			r.attempts[address] = attempt
		}
	}
	// 只保留仍可选择的地址；实例退出后不能让空请求继续占用观察额度。
	for address, attempt := range r.attempts {
		if attempt.round != r.scheduleRound {
			delete(r.attempts, address)
			delete(r.pending, address)
		}
	}
	limit = min(max(limit, 0), len(peers))
	available := min(limit, max(limit-len(r.pending), 0))
	r.candidates = r.candidates[:0]
	return limit, available
}

// insertProbeCandidate 只保留额度大小的有序候选，避免排序整个地址集合。
func insertProbeCandidate(candidates []probeCandidate, candidate probeCandidate, limit int) []probeCandidate {
	if len(candidates) == limit && candidate.sequence >= candidates[limit-1].sequence {
		return candidates
	}
	if len(candidates) < limit {
		candidates = append(candidates, probeCandidate{})
	}
	position := len(candidates) - 1
	for position > 0 && candidate.sequence < candidates[position-1].sequence {
		candidates[position] = candidates[position-1]
		position--
	}
	candidates[position] = candidate
	return candidates
}

// prioritizePeers 将获得观察额度的地址移到前方，其余名额保留随机 Gossip。
func (r *localReachability) prioritizePeers(peers []vivid.ActorRef, limit int) []vivid.ActorRef {
	for selected := range r.candidates {
		index := r.candidates[selected].index
		peers[selected], peers[index] = peers[index], peers[selected]
		// 交换可能移动另一个已选候选，更新其下标，不额外复制引用列表。
		for next := selected + 1; next < len(r.candidates); next++ {
			if r.candidates[next].index == selected {
				r.candidates[next].index = index
				break
			}
		}
	}
	return peers[:limit]
}
