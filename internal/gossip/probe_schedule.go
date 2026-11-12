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
	if available == 0 {
		return peers[:limit]
	}
	for index, peer := range peers {
		address := peer.GetAddress()
		if _, waiting := r.pending[address]; waiting {
			continue
		}
		sequence := r.attempts[address].sequence
		if len(r.candidates) == available && sequence >= r.candidates[available-1].sequence {
			continue
		}
		if len(r.candidates) < available {
			r.candidates = append(r.candidates, probeCandidate{})
		}
		// 仅维护空闲额度大小的有序候选，不为每个名额重复查询所有地址。
		position := len(r.candidates) - 1
		for position > 0 && sequence < r.candidates[position-1].sequence {
			r.candidates[position] = r.candidates[position-1]
			position--
		}
		r.candidates[position] = probeCandidate{index: index, sequence: sequence}
	}
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
