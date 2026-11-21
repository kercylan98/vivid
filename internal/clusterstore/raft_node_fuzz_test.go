package clusterstore

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func FuzzRaftNode_Network(f *testing.F) {
	f.Add([]byte{0, 0, 0, 0, 0, 1, 1, 0, 2, 0, 0, 0})
	f.Add([]byte{0, 1, 1, 1, 3, 2, 0, 0, 4, 1, 2, 0, 0, 3, 0, 0})
	f.Add([]byte{0, 0, 0, 0, 1, 0, 0, 0, 7, 0, 0, 0, 8, 0, 0, 0})
	// 先完成选举，再隔离副本、推进提交、压缩并重发快照。
	seed := []byte{0, 0, 0, 0}
	for range 12 {
		seed = append(seed, 2, 0, 0, 0)
	}
	seed = append(seed, 3, 4, 0, 0)
	for range 24 {
		seed = append(seed, 1, 0, 0, 0)
	}
	seed = append(seed, 7, 0, 0, 0, 8, 0, 0, 0, 3, 4, 0, 0)
	for range 12 {
		seed = append(seed, 8, 0, 0, 0)
	}
	f.Add(seed)
	f.Fuzz(func(t *testing.T, operations []byte) {
		if len(operations) > 1024 {
			return
		}
		network := newRaftNetwork(t, 5)
		for step := 0; step+3 < len(operations); step += 4 {
			id := uint64(operations[step+1]%5 + 1)
			node := network.nodes[id]
			choice := int(operations[step+2])<<8 | int(operations[step+3])
			switch operations[step] % 9 {
			case 0:
				if node.role != RaftLeader {
					network.campaign(t, id)
				}
			case 1:
				if node.role == RaftLeader {
					_, out, err := node.Propose(operations[step : step+4])
					require.NoError(t, err)
					network.queue = append(network.queue, out...)
				}
			case 2:
				if node.role == RaftLeader {
					out, err := node.Heartbeat()
					require.NoError(t, err)
					network.queue = append(network.queue, out...)
				}
			case 3:
				network.blocked[id] = !network.blocked[id]
			case 4:
				var err error
				network.nodes[id], err = newRaftNode(network.disks[id])
				require.NoError(t, err)
			case 5:
				if len(network.queue) > 0 && len(network.queue) < 1000 {
					network.queue = append(network.queue, network.queue[choice%len(network.queue)])
				}
			case 6:
				if len(network.queue) > 0 {
					index := choice % len(network.queue)
					network.queue = append(network.queue[:index], network.queue[index+1:]...)
				}
			case 7:
				if node.hard.Commit > network.disks[id].state.baseIndex {
					store := Store{index: node.hard.Commit}
					require.NoError(t, node.Compact(store.Snapshot()))
				}
			case 8:
				if node.role == RaftLeader {
					for _, peerID := range node.config.Voters {
						if peer := node.peers[peerID]; peer != nil && peer.snapshot {
							out, err := node.SendSnapshot(peerID)
							require.NoError(t, err)
							network.queue = append(network.queue, out...)
						}
					}
				}
			}
			if len(network.queue) > 0 {
				network.deliver(t, choice%len(network.queue))
			}
			network.check(t)
		}
		leader := network.recover(t)
		index, out, err := leader.Propose([]byte("recovered"))
		require.NoError(t, err)
		network.queue = out
		network.drain(t)
		for _, node := range network.nodes {
			require.Equal(t, index, node.hard.Commit)
		}
	})
}
