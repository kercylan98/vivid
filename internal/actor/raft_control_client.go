package actor

import (
	"context"
	"crypto/sha256"
	"errors"
	"slices"
	"sync/atomic"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/clusterstore"
)

// raftControlClient 只缓存固定成员的联系线索，不缓存归属地址或多数派授权。
// 每次联络最多遍历一次成员；只有明确的提案前非 Leader 拒绝才允许另寻入口。
type raftControlClient struct {
	system  *System
	group   [sha256.Size]byte
	routes  map[uint64]vivid.ActorRef
	peers   []uint64
	leader  atomic.Uint64
	contact time.Duration
}

func newRaftControlClient(system *System, config clusterstore.RaftConfig, routes map[uint64]vivid.ActorRef) (*raftControlClient, error) {
	group, err := clusterstore.RaftControlGroup(config)
	if err != nil || system == nil || len(routes) != len(config.Voters) {
		return nil, vivid.ErrorIllegalArgument
	}
	client := &raftControlClient{system: system, group: group, routes: make(map[uint64]vivid.ActorRef, len(routes)), peers: slices.Clone(config.Voters), contact: 200 * time.Millisecond}
	slices.Sort(client.peers)
	addresses := make(map[string]bool, len(routes))
	for _, id := range client.peers {
		ref := routes[id]
		if ref == nil || ref.IsVirtual() || ref.GetAddress() == "" || len(ref.String()) > 4<<10 || addresses[ref.GetAddress()] {
			return nil, vivid.ErrorIllegalArgument
		}
		addresses[ref.GetAddress()] = true
		client.routes[id] = ref.Clone()
	}
	return client, nil
}

func (c *raftControlClient) ask(ctx context.Context, node uint64, request *clusterstore.RaftControlRequest) (*clusterstore.RaftControlResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	request.Group, request.Node, request.Timeout = c.group, node, ownershipTimeout(ctx)
	future := c.system.Ask(c.routes[node], request, 0)
	stop := context.AfterFunc(ctx, func() { future.Close(ctx.Err()) })
	defer stop()
	result, err := future.Result()
	if err != nil {
		return nil, err
	}
	response, ok := result.(*clusterstore.RaftControlResponse)
	if !ok || response == nil || response.Group != c.group || response.Node != node || response.Operation != request.Operation || response.Role > clusterstore.RaftLeader || response.Leader != 0 && c.routes[response.Leader] == nil {
		return nil, clusterstore.ErrInvalidRaftControl
	}
	return response, nil
}

func (c *raftControlClient) call(ctx context.Context, operation clusterstore.RaftControlOperation, command clusterstore.Command, key string) (*clusterstore.RaftControlResponse, error) {
	// 调用方没有期限时仍使用一次有限等待，正常虚拟生命周期传入更短的剩余预算。
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}
	visited := make(map[uint64]bool, len(c.peers))
	candidate := c.leader.Load()
	var last error = clusterstore.ErrRaftNotLeader
	for len(visited) < len(c.peers) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if candidate == 0 || visited[candidate] || c.routes[candidate] == nil {
			candidate = 0
			for _, peer := range c.peers {
				if !visited[peer] {
					candidate = peer
					break
				}
			}
		}
		visited[candidate] = true
		// 定位超时不包含业务写入，可在同一总预算内继续联系其他成员。
		contact, cancel := context.WithTimeout(ctx, c.contact)
		location, err := c.ask(contact, candidate, &clusterstore.RaftControlRequest{Operation: clusterstore.RaftControlLocate})
		cancel()
		if err != nil {
			last, candidate = err, 0
			continue
		}
		if err = location.Err(); err != nil {
			last, candidate = err, 0
			continue
		}
		if location.Role != clusterstore.RaftLeader {
			candidate = location.Leader
			continue
		}
		response, err := c.ask(ctx, candidate, &clusterstore.RaftControlRequest{Operation: operation, Command: command, Key: key})
		if err != nil {
			c.leader.CompareAndSwap(candidate, 0)
			if operation == clusterstore.RaftControlExecute {
				// 传输、解码及外层超时均无法证明远端尚未接纳，不重新发送写入。
				err = errors.Join(clusterstore.ErrRaftOutcomeUnknown, err)
			}
			return nil, err
		}
		if errors.Is(response.Err(), clusterstore.ErrRaftNotLeader) {
			if operation == clusterstore.RaftControlExecute {
				execution := response.Execution()
				if execution.Index != 0 || execution.Term != 0 || execution.Result.Applied || execution.Result.Revision != 0 || execution.Result.Err != nil {
					return nil, errors.Join(clusterstore.ErrRaftOutcomeUnknown, clusterstore.ErrInvalidRaftControl)
				}
			}
			c.leader.CompareAndSwap(candidate, 0)
			last, candidate = response.Err(), response.Leader
			continue
		}
		if response.Err() == nil {
			c.leader.Store(candidate)
		}
		return response, nil
	}
	return nil, last
}
