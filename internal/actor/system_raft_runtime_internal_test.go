package actor

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/clusterstore"
	"github.com/kercylan98/vivid/internal/remoting"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/require"
)

type raftRuntimeLink struct {
	front, back net.Conn
	from, to    uint64
}

type raftRuntimeNetwork struct {
	mu         sync.Mutex
	addresses  map[string]uint64
	isolated   map[uint64]bool
	links      map[*raftRuntimeLink]bool
	generation uint64
	closed     bool
	rejected   int
	wg         sync.WaitGroup
	listeners  []net.Listener
	systems    map[uint64]*System
	refs       map[uint64]vivid.ActorRef
	paths      map[uint64]string
	backends   map[uint64]string
	budgets    map[uint64]*clusterstore.SnapshotTransferBudget
}

func (n *raftRuntimeNetwork) partition(ids ...uint64) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.generation++
	clear(n.isolated)
	for _, id := range ids {
		n.isolated[id] = true
	}
	for link := range n.links {
		if n.isolated[link.from] || n.isolated[link.to] {
			_ = link.front.Close()
			_ = link.back.Close()
		}
	}
}

func (n *raftRuntimeNetwork) proxy(listener net.Listener, backend string, id uint64) {
	defer n.wg.Done()
	for {
		front, err := listener.Accept()
		if err != nil {
			return
		}
		n.wg.Add(1)
		go n.forward(front, backend, id)
	}
}

func (n *raftRuntimeNetwork) forward(front net.Conn, backend string, id uint64) {
	defer n.wg.Done()
	defer front.Close()
	_ = front.SetReadDeadline(time.Now().Add(time.Second))
	frame, err := remoting.ReadFrame(front, nil, remoting.FrameReadLimits{MaxDataLen: 1024})
	if err != nil || frame.Type != remoting.FrameCtrlHandshake {
		return
	}
	_ = front.SetReadDeadline(time.Time{})
	n.mu.Lock()
	from, generation := n.addresses[string(frame.Data)], n.generation
	blocked := n.closed || from == 0 || n.isolated[from] || n.isolated[id]
	if blocked {
		n.rejected++
	}
	n.mu.Unlock()
	if blocked {
		return
	}
	back, err := net.DialTimeout("tcp", backend, time.Second)
	if err != nil {
		return
	}
	defer back.Close()
	link := &raftRuntimeLink{front: front, back: back, from: from, to: id}
	n.mu.Lock()
	if n.closed || generation != n.generation || n.isolated[from] || n.isolated[id] {
		n.mu.Unlock()
		return
	}
	n.links[link] = true
	n.mu.Unlock()
	defer func() { n.mu.Lock(); delete(n.links, link); n.mu.Unlock() }()
	if _, err := frame.WriteTo(back); err != nil {
		return
	}
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(back, front)
		_ = front.Close()
		_ = back.Close()
		close(done)
	}()
	_, _ = io.Copy(front, back)
	_ = front.Close()
	_ = back.Close()
	<-done
}

func newRaftRuntimeNetwork(t *testing.T, isolated bool, prepare func(uint64, string)) *raftRuntimeNetwork {
	t.Helper()
	n := &raftRuntimeNetwork{addresses: make(map[string]uint64), isolated: make(map[uint64]bool), links: make(map[*raftRuntimeLink]bool), systems: make(map[uint64]*System), refs: make(map[uint64]vivid.ActorRef), paths: make(map[uint64]string), backends: make(map[uint64]string), budgets: make(map[uint64]*clusterstore.SnapshotTransferBudget)}
	for id := uint64(1); id <= 3; id++ {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		n.listeners = append(n.listeners, listener)
		n.addresses[listener.Addr().String()] = id
		n.refs[id], err = NewRef(listener.Addr().String(), "/raft")
		require.NoError(t, err)
		n.backends[id] = admissionSystemAddress(t)
		n.paths[id] = filepath.Join(t.TempDir(), "node.wal")
		if isolated {
			n.isolated[id] = true
		}
		if prepare != nil {
			prepare(id, n.paths[id])
		}
	}
	t.Cleanup(func() {
		n.partition()
		for _, system := range n.systems {
			require.NoError(t, system.Stop(3*time.Second))
		}
		n.mu.Lock()
		n.closed = true
		for link := range n.links {
			_ = link.front.Close()
			_ = link.back.Close()
		}
		n.mu.Unlock()
		for _, listener := range n.listeners {
			_ = listener.Close()
		}
		n.wg.Wait()
	})
	for i, listener := range n.listeners {
		id := uint64(i + 1)
		n.wg.Add(1)
		go n.proxy(listener, n.backends[id], id)
	}
	for id := uint64(1); id <= 3; id++ {
		n.start(t, id, prepare == nil)
	}
	return n
}

func (n *raftRuntimeNetwork) start(t *testing.T, id uint64, create bool) {
	t.Helper()
	system := NewSystem(
		vivid.WithActorSystemRemoting(n.backends[id], n.refs[id].GetAddress()),
		vivid.WithActorSystemRemotingOption(vivid.WithActorSystemRemotingReconnect(2, 20*time.Millisecond, 40*time.Millisecond, 2, false)),
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
	)
	require.NoError(t, system.Start())
	n.systems[id] = system
	budget, err := clusterstore.NewSnapshotTransferBudget(clusterstore.SnapshotTransferLimits{})
	require.NoError(t, err)
	n.budgets[id] = budget
	driver, err := clusterstore.NewRaftActor(clusterstore.RaftActorOptions{
		Path: n.paths[id], Config: clusterstore.RaftConfig{GroupID: "runtime", NodeID: id, Voters: []uint64{1, 2, 3}}, Create: create, Routes: n.refs,
		HeartbeatInterval: 20 * time.Millisecond, ElectionMin: 200 * time.Millisecond, ElectionMax: 400 * time.Millisecond, SnapshotBudget: budget,
	})
	require.NoError(t, err)
	ref, err := system.ActorOf(driver, vivid.WithActorName("raft"))
	require.NoError(t, err)
	require.True(t, n.refs[id].Equals(ref))
	n.refs[id] = ref
}

func (n *raftRuntimeNetwork) status(t *testing.T, id uint64) clusterstore.RaftRuntimeStatus {
	t.Helper()
	result, err := n.systems[id].Ask(n.refs[id], clusterstore.RaftRuntimeStatusRequest{}, time.Second).Result()
	require.NoError(t, err)
	status := result.(clusterstore.RaftRuntimeStatus)
	require.NoError(t, status.Err)
	require.LessOrEqual(t, status.PendingBytes, uint64(16<<20))
	return status
}

func (n *raftRuntimeNetwork) leader(t *testing.T, ids ...uint64) uint64 {
	t.Helper()
	var leader uint64
	require.Eventually(t, func() bool {
		leader = 0
		for _, id := range ids {
			status := n.status(t, id)
			if status.Role == clusterstore.RaftLeader {
				if leader != 0 {
					return false
				}
				leader = id
			}
		}
		return leader != 0
	}, 8*time.Second, 20*time.Millisecond)
	return leader
}

func (n *raftRuntimeNetwork) propose(t *testing.T, id uint64, value string) uint64 {
	t.Helper()
	result, err := n.systems[id].Ask(n.refs[id], clusterstore.RaftRuntimePropose{Data: []byte(value)}, time.Second).Result()
	require.NoError(t, err)
	accepted := result.(clusterstore.RaftRuntimeResult)
	require.NoError(t, accepted.Err)
	return accepted.Index
}

func (n *raftRuntimeNetwork) committed(t *testing.T, index uint64, ids ...uint64) {
	t.Helper()
	require.Eventually(t, func() bool {
		for _, id := range ids {
			if n.status(t, id).Commit < index {
				return false
			}
		}
		return true
	}, 8*time.Second, 20*time.Millisecond)
}

func TestSystem_RaftRuntimePartitionRecovery(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	old := n.leader(t, 1, 2, 3)
	index := n.propose(t, old, "first")
	n.committed(t, index, 1, 2, 3)
	n.partition(old)
	var majority []uint64
	for id := uint64(1); id <= 3; id++ {
		if id != old {
			majority = append(majority, id)
		}
	}
	next := n.leader(t, majority...)
	minority := n.propose(t, old, "minority-uncommitted")
	require.Less(t, n.status(t, old).Commit, minority, "旧 Leader 的本地接纳不能冒充多数派提交")
	index = n.propose(t, next, "majority")
	n.committed(t, index, majority...)
	n.partition()
	n.committed(t, index, 1, 2, 3)
	for id := uint64(1); id <= 3; id++ {
		result, err := n.systems[id].Ask(n.refs[id], clusterstore.RaftRuntimeRead{}, time.Second).Result()
		require.NoError(t, err)
		log := result.(clusterstore.RaftRuntimeLog)
		require.NoError(t, log.Err)
		var values []string
		for _, entry := range log.Entries {
			if len(entry.Data) != 0 {
				values = append(values, string(entry.Data))
			}
		}
		require.Equal(t, []string{"first", "majority"}, values)
	}
	for id, system := range n.systems {
		require.NoError(t, system.Stop(3*time.Second))
		delete(n.systems, id)
	}
	for id := uint64(1); id <= 3; id++ {
		n.start(t, id, false)
	}
	_ = n.leader(t, 1, 2, 3)
	n.committed(t, index, 1, 2, 3)
}

func TestSystem_RaftRuntimeIsolatedBootstrap(t *testing.T) {
	n := newRaftRuntimeNetwork(t, true, nil)
	require.Eventually(t, func() bool {
		for id := uint64(1); id <= 3; id++ {
			status := n.status(t, id)
			if status.Term < 2 || status.Role == clusterstore.RaftLeader || status.Commit != 0 {
				return false
			}
		}
		n.mu.Lock()
		defer n.mu.Unlock()
		return n.rejected > 0
	}, 5*time.Second, 20*time.Millisecond)
	n.partition()
	leader := n.leader(t, 1, 2, 3)
	index := n.propose(t, leader, "after-recovery")
	n.committed(t, index, 1, 2, 3)
}

func TestSystem_RaftRuntimeLargeSnapshot(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, func(id uint64, path string) {
		config := clusterstore.RaftConfig{GroupID: "runtime", NodeID: id, Voters: []uint64{1, 2, 3}}
		storage, err := clusterstore.CreateRaftStorage(path, config)
		require.NoError(t, err)
		defer storage.Close()
		if id == 3 {
			return
		}
		var store clusterstore.Store
		var entries []clusterstore.RaftEntry
		for i := uint64(1); i <= 6; i++ {
			command := clusterstore.Command{}
			if i > 1 {
				command = clusterstore.Command{Operation: clusterstore.Put, Key: fmt.Sprint(i), Value: bytes.Repeat([]byte{byte(i)}, 1<<20)}
			}
			_, err := store.Apply(i, command)
			require.NoError(t, err)
			entries = append(entries, clusterstore.RaftEntry{Index: i, Term: 1})
		}
		require.NoError(t, storage.Save(clusterstore.RaftUpdate{HardState: clusterstore.RaftHardState{Term: 1, Vote: 1, Commit: 6}, Entries: entries}))
		require.NoError(t, storage.Compact(store.Snapshot()))
	})
	leader := n.leader(t, 1, 2)
	index := n.propose(t, leader, "snapshot-suffix")
	n.committed(t, index, 1, 2, 3)
	require.EqualValues(t, 6, n.status(t, 3).BaseIndex)
	result, err := n.systems[3].Ask(n.refs[3], clusterstore.RaftRuntimeRead{}, time.Second).Result()
	require.NoError(t, err)
	log := result.(clusterstore.RaftRuntimeLog)
	require.NoError(t, log.Err)
	require.NotNil(t, log.Snapshot)
	require.Greater(t, len(log.Snapshot.Data), 4<<20)
	state, err := clusterstore.Restore(log.Snapshot.Data)
	require.NoError(t, err)
	for i := uint64(2); i <= 6; i++ {
		record, exists := state.Get(fmt.Sprint(i))
		require.True(t, exists)
		require.Equal(t, bytes.Repeat([]byte{byte(i)}, 1<<20), record.Value)
	}
	require.Eventually(t, func() bool {
		for _, budget := range n.budgets {
			if budget.Usage() != (clusterstore.SnapshotTransferUsage{}) {
				return false
			}
		}
		return true
	}, 3*time.Second, 20*time.Millisecond)
}
