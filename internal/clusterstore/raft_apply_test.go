package clusterstore

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func proposeCommand(t testing.TB, node *RaftNode, command Command) uint64 {
	t.Helper()
	data, err := EncodeCommand(command)
	require.NoError(t, err)
	index, _, err := node.Propose(data)
	require.NoError(t, err)
	return index
}

func TestRaftApply_CompareAndReplay(t *testing.T) {
	network := newRaftNetwork(t, 1)
	network.campaign(t, 1)
	node := network.nodes[1]
	for _, command := range []Command{
		{Operation: Put, Key: "owner", Value: []byte("a")},
		{Operation: Put, Key: "owner", Value: []byte("conflict")},
		{Operation: Delete, Key: "owner", ExpectedRevision: 2},
		{Operation: Put, Key: "owner", Value: []byte("b")},
	} {
		proposeCommand(t, node, command)
	}
	store, err := replayRaftStore(node, &Store{}, nil)
	require.NoError(t, err)
	require.EqualValues(t, 5, store.Index(), "空操作及 CAS 失败也消费日志位置")
	record, exists := store.Get("owner")
	require.True(t, exists)
	require.Equal(t, Record{Value: []byte("b"), Revision: 5}, record)
	require.NoError(t, node.Compact(store.Snapshot()))
	// 后缀超过一次 Committed 的条数限制，恢复须继续取后续批次。
	for i := range 70 {
		proposeCommand(t, node, Command{Operation: Put, Key: fmt.Sprint(i), Value: []byte("suffix")})
	}
	store, err = replayRaftStore(node, store, nil)
	require.NoError(t, err)
	restored, err := replayRaftStore(node, &Store{}, nil)
	require.NoError(t, err)
	require.Equal(t, store.Snapshot(), restored.Snapshot())
	require.EqualValues(t, 75, restored.Index())
	unchanged, err := replayRaftStore(node, restored, nil)
	require.NoError(t, err)
	require.Same(t, restored, unchanged)
}

func TestRaftApply_CommittedOnly(t *testing.T) {
	network := newRaftNetwork(t, 3)
	network.campaign(t, 1)
	network.drain(t)
	index := proposeCommand(t, network.nodes[1], Command{Operation: Put, Key: "owner", Value: []byte("minority")})
	store, err := replayRaftStore(network.nodes[1], &Store{}, nil)
	require.NoError(t, err)
	require.Less(t, store.Index(), index)
	_, exists := store.Get("owner")
	require.False(t, exists)
}

func TestRaftApply_InvalidCommittedCommand(t *testing.T) {
	network := newRaftNetwork(t, 1)
	network.campaign(t, 1)
	node := network.nodes[1]
	proposeCommand(t, node, Command{Operation: Put, Key: "owner", Value: []byte("a")})
	_, _, err := node.Propose([]byte("invalid"))
	require.NoError(t, err)
	proposeCommand(t, node, Command{Operation: Put, Key: "skipped", Value: []byte("b")})
	store, err := replayRaftStore(node, &Store{}, nil)
	require.ErrorIs(t, err, ErrInvalidCommand)
	require.EqualValues(t, 2, store.Index())
	_, exists := store.Get("skipped")
	require.False(t, exists, "不能跳过格式错误继续消费后续提交")
}

func TestRaftApply_CapacityContinues(t *testing.T) {
	network := newRaftNetwork(t, 1)
	network.campaign(t, 1)
	node := network.nodes[1]
	value := bytes.Repeat([]byte("v"), maxValueBytes)
	for i := range 32 {
		proposeCommand(t, node, Command{Operation: Put, Key: fmt.Sprint(i), Value: value})
	}
	proposeCommand(t, node, Command{Operation: Put, Key: "small", Value: []byte("still fits")})
	store, err := replayRaftStore(node, &Store{}, nil)
	require.NoError(t, err, "容量不足属于确定性的业务结果，不能停止应用")
	require.EqualValues(t, 34, store.Index())
	_, exists := store.Get("31")
	require.False(t, exists)
	record, exists := store.Get("small")
	require.True(t, exists)
	require.Equal(t, []byte("still fits"), record.Value)
}

func BenchmarkRaftApply_CaughtUp(b *testing.B) {
	network := newRaftNetwork(b, 1)
	network.campaign(b, 1)
	store, err := replayRaftStore(network.nodes[1], &Store{}, nil)
	require.NoError(b, err)
	driver := &RaftActor{node: network.nodes[1], store: store, applied: store.Index(), lifetime: &raftActorLifetime{}}
	b.ReportAllocs()
	for b.Loop() {
		driver.apply(nil)
	}
}

func BenchmarkRaftApply_Replay(b *testing.B) {
	for _, count := range []int{1, 256} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			network := newRaftNetwork(b, 1)
			network.campaign(b, 1)
			node := network.nodes[1]
			for i := range count {
				proposeCommand(b, node, Command{Operation: Put, Key: fmt.Sprint(i), Value: make([]byte, 64)})
			}
			b.ReportAllocs()
			for b.Loop() {
				store, err := replayRaftStore(node, &Store{}, nil)
				if err != nil || store.Index() != uint64(count+1) {
					b.Fatalf("应用位置不符: %d, %v", store.Index(), err)
				}
			}
		})
	}
}
