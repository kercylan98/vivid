package clusterstore

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func applyVirtualOwnership(t testing.TB, store *Store, change VirtualOwnershipChange) (VirtualOwnership, error) {
	t.Helper()
	data, err := EncodeCommand(change.Request(0).Command)
	require.NoError(t, err)
	command, err := DecodeCommand(data)
	require.NoError(t, err)
	index := store.Index() + 1
	result, err := store.Apply(index, command)
	require.NoError(t, err)
	return change.Result(RaftRuntimeExecution{Index: index, Term: 1, Result: result})
}

func readVirtualOwnership(t testing.TB, store *Store, kind, name string) (VirtualOwnership, bool) {
	t.Helper()
	key, err := VirtualOwnershipKey(kind, name)
	require.NoError(t, err)
	record, exists := store.Get(key)
	ownership, exists, err := ReadVirtualOwnership(kind, name, RaftRuntimeRecord{Record: record, Exists: exists, Term: 1, Index: store.Index()})
	require.NoError(t, err)
	return ownership, exists
}

func TestVirtualOwnership_CompetitionReleaseAndReacquire(t *testing.T) {
	owner, err := NewVirtualOwner("127.0.0.1:9000")
	require.NoError(t, err)
	restarted, err := NewVirtualOwner(owner.Address)
	require.NoError(t, err)
	require.NotEqual(t, owner.InstanceID, restarted.InstanceID, "同地址重启必须具有新启动身份")
	first, err := ClaimVirtualOwnership("counter", "one", owner, 0)
	require.NoError(t, err)
	competitor, err := ClaimVirtualOwnership("counter", "one", restarted, 0)
	require.NoError(t, err)
	var store Store
	claimed, err := applyVirtualOwnership(t, &store, first)
	require.NoError(t, err)
	require.EqualValues(t, 1, claimed.Generation)
	loser, err := applyVirtualOwnership(t, &store, competitor)
	require.ErrorIs(t, err, ErrVirtualOwnershipConflict)
	require.Zero(t, loser)
	current, exists := readVirtualOwnership(t, &store, "counter", "one")
	require.True(t, exists)
	require.Equal(t, claimed, current)
	oldRelease, err := claimed.Release()
	require.NoError(t, err)
	released, err := applyVirtualOwnership(t, &store, oldRelease)
	require.NoError(t, err)
	require.Zero(t, released)
	_, exists = readVirtualOwnership(t, &store, "counter", "one")
	require.False(t, exists)
	newOwnership, err := applyVirtualOwnership(t, &store, competitor)
	require.NoError(t, err)
	require.Greater(t, newOwnership.Generation, claimed.Generation)
	require.Equal(t, restarted, newOwnership.Owner)
	_, err = applyVirtualOwnership(t, &store, oldRelease)
	require.ErrorIs(t, err, ErrVirtualOwnershipConflict, "旧释放不能删除同地址的新启动归属")
	staleTransfer, err := ClaimVirtualOwnership("counter", "one", owner, claimed.Generation)
	require.NoError(t, err)
	_, err = applyVirtualOwnership(t, &store, staleTransfer)
	require.ErrorIs(t, err, ErrVirtualOwnershipConflict)
	current, exists = readVirtualOwnership(t, &store, "counter", "one")
	require.True(t, exists)
	require.Equal(t, newOwnership, current)
}

func TestVirtualOwnership_TransferAndSnapshot(t *testing.T) {
	owner, err := NewVirtualOwner("a.example:9000")
	require.NoError(t, err)
	nextOwner, err := NewVirtualOwner("[::1]:9001")
	require.NoError(t, err)
	claim, err := ClaimVirtualOwnership("a/b", "订单", owner, 0)
	require.NoError(t, err)
	var store Store
	first, err := applyVirtualOwnership(t, &store, claim)
	require.NoError(t, err)
	transfer, err := ClaimVirtualOwnership(first.Kind, first.Name, nextOwner, first.Generation)
	require.NoError(t, err)
	current, err := applyVirtualOwnership(t, &store, transfer)
	require.NoError(t, err)
	require.Greater(t, current.Generation, first.Generation)
	restored, err := Restore(store.Snapshot())
	require.NoError(t, err)
	got, exists := readVirtualOwnership(t, restored, first.Kind, first.Name)
	require.True(t, exists)
	require.Equal(t, current, got)
	release, err := current.Release()
	require.NoError(t, err)
	_, err = applyVirtualOwnership(t, restored, release)
	require.NoError(t, err)
	restored, err = Restore(restored.Snapshot())
	require.NoError(t, err)
	reacquired, err := applyVirtualOwnership(t, restored, claim)
	require.NoError(t, err)
	require.Greater(t, reacquired.Generation, current.Generation, "释放后的快照恢复不能回退代次")
	key, err := VirtualOwnershipKey("a", "b/订单")
	require.NoError(t, err)
	require.NotEqual(t, claim.key, key)
	_, exists = readVirtualOwnership(t, restored, "a", "b/订单")
	require.False(t, exists)
}

func TestVirtualOwnership_IndependentRequestAndReturnedValue(t *testing.T) {
	owner, err := NewVirtualOwner("node.example:9000")
	require.NoError(t, err)
	claim, err := ClaimVirtualOwnership("kind", "name", owner, 0)
	require.NoError(t, err)
	request := claim.Request(time.Second)
	require.Equal(t, time.Second, request.Timeout)
	clear(request.Command.Value)
	var store Store
	ownership, err := applyVirtualOwnership(t, &store, claim)
	require.NoError(t, err, "外部修改一次发送负载不能改变后续构造的请求")
	record, _ := store.Get(claim.key)
	got, exists, err := ReadVirtualOwnership("kind", "name", RaftRuntimeRecord{Record: record, Exists: true, Index: store.Index(), Term: 1})
	require.NoError(t, err)
	require.True(t, exists)
	clear(record.Value)
	require.Equal(t, ownership, got, "解析结果不得借用消息负载")
	got.Owner.Address = "changed:9000"
	again, _ := readVirtualOwnership(t, &store, "kind", "name")
	require.Equal(t, ownership, again)
}

func TestVirtualOwnership_UnknownAndRejectedExecution(t *testing.T) {
	owner, err := NewVirtualOwner("node.example:9000")
	require.NoError(t, err)
	claim, err := ClaimVirtualOwnership("kind", "name", owner, 3)
	require.NoError(t, err)
	for _, tc := range []struct {
		result RaftRuntimeExecution
		want   error
	}{
		{RaftRuntimeExecution{Index: 9, Term: 2, Err: errors.Join(ErrRaftOutcomeUnknown, ErrRaftExecutionTimeout)}, ErrRaftOutcomeUnknown},
		{RaftRuntimeExecution{Err: ErrRaftNotLeader}, ErrRaftNotLeader},
		{RaftRuntimeExecution{Result: Result{Err: ErrCapacity}}, ErrCapacity},
		{RaftRuntimeExecution{Index: 9, Term: 2, Result: Result{Revision: 7}}, ErrVirtualOwnershipConflict},
		{RaftRuntimeExecution{Index: 9, Result: Result{Applied: true, Revision: 9}}, ErrInvalidVirtualOwnership},
		{RaftRuntimeExecution{Term: 2, Result: Result{Applied: true}}, ErrInvalidVirtualOwnership},
		{RaftRuntimeExecution{Index: 3, Term: 2, Result: Result{Applied: true, Revision: 3}}, ErrInvalidVirtualOwnership},
		{RaftRuntimeExecution{Index: 9, Term: 2, Result: Result{Applied: true, Revision: 8}}, ErrInvalidVirtualOwnership},
	} {
		got, err := claim.Result(tc.result)
		require.ErrorIs(t, err, tc.want)
		require.Zero(t, got, "任何不确定或失败结果均不能生成归属代次")
	}
	var zero VirtualOwnershipChange
	_, err = zero.Result(RaftRuntimeExecution{Index: 9, Term: 2, Result: Result{Applied: true, Revision: 9}})
	require.ErrorIs(t, err, ErrInvalidVirtualOwnership)
}

func TestVirtualOwnership_InvalidIdentityOwnerAndRecord(t *testing.T) {
	owner, err := NewVirtualOwner("node.example:9000")
	require.NoError(t, err)
	for _, identity := range [][2]string{{"", "name"}, {"kind", ""}, {string([]byte{255}), "name"}, {"kind", string([]byte{255})}, {strings.Repeat("a", maxKeyBytes), "b"}} {
		_, err := ClaimVirtualOwnership(identity[0], identity[1], owner, 0)
		require.ErrorIs(t, err, ErrInvalidVirtualOwnership)
	}
	for _, address := range []string{"", "localhost", ":9000", "host:0", "host:65536", "host:abc", "host:9000/path", "host :9000", "host:\n9000", "ho\x01st:9000", "user@host:9000", strings.Repeat("a", maxVirtualAddressBytes) + ":9000"} {
		_, err := NewVirtualOwner(address)
		require.ErrorIs(t, err, ErrInvalidVirtualOwnership, address)
	}
	_, err = ClaimVirtualOwnership("kind", "name", VirtualOwner{Address: owner.Address}, 0)
	require.ErrorIs(t, err, ErrInvalidVirtualOwnership)
	_, err = (VirtualOwnership{Kind: "kind", Name: "name", Owner: owner}).Release()
	require.ErrorIs(t, err, ErrInvalidVirtualOwnership)
	data := encodeVirtualOwner(owner)
	for size := range len(data) {
		_, err := decodeVirtualOwner(data[:size])
		require.ErrorIs(t, err, ErrInvalidVirtualOwnership)
	}
	for _, mutate := range []func([]byte) []byte{
		func(data []byte) []byte { data[0] ^= 1; return data },
		func(data []byte) []byte { clear(data[8:24]); return data },
		func(data []byte) []byte { binary.LittleEndian.PutUint32(data[24:], ^uint32(0)); return data },
		func(data []byte) []byte { data[len(data)-1] = 0; return data },
		func(data []byte) []byte { return append(data, 0) },
	} {
		_, err := decodeVirtualOwner(mutate(bytes.Clone(data)))
		require.ErrorIs(t, err, ErrInvalidVirtualOwnership)
	}
	valid := RaftRuntimeRecord{Record: Record{Value: data, Revision: 2}, Exists: true, Index: 3, Term: 1}
	for _, mutate := range []func(*RaftRuntimeRecord){
		func(record *RaftRuntimeRecord) { record.Term = 0 },
		func(record *RaftRuntimeRecord) { record.Index = 0 },
		func(record *RaftRuntimeRecord) { record.Revision = 0 },
		func(record *RaftRuntimeRecord) { record.Revision = record.Index + 1 },
		func(record *RaftRuntimeRecord) { record.Exists = false },
	} {
		record := valid
		mutate(&record)
		got, exists, err := ReadVirtualOwnership("kind", "name", record)
		require.ErrorIs(t, err, ErrInvalidVirtualOwnership)
		require.False(t, exists)
		require.Zero(t, got)
	}
	valid.Err = ErrRaftReadTimeout
	got, exists, err := ReadVirtualOwnership("kind", "name", valid)
	require.ErrorIs(t, err, ErrRaftReadTimeout)
	require.False(t, exists)
	require.Zero(t, got)
}

func FuzzVirtualOwnership_Decode(f *testing.F) {
	owner := VirtualOwner{InstanceID: [16]byte{1}, Address: "node.example:9000"}
	f.Add(encodeVirtualOwner(owner))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1024 {
			return
		}
		owner, err := decodeVirtualOwner(data)
		if err == nil {
			require.Equal(t, data, encodeVirtualOwner(owner))
		}
	})
}
