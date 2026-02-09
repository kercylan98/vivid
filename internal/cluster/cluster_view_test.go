package cluster

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testCaseTimeout = 10 * time.Second

func TestClusterView_AddMember_RemoveMember_QuorumSize(t *testing.T) {
	deadline := time.Now().Add(testCaseTimeout)
	if d, ok := t.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	t.Cleanup(func() {
		if time.Now().After(deadline) {
			t.Error("test exceeded timeout")
		}
	})

	v := newClusterView()
	require.NotNil(t, v)
	assert.Equal(t, 0, v.HealthyCount)
	assert.Equal(t, 0, v.QuorumSize)

	n1 := newNodeState("n1", "c1", "127.0.0.1:8001")
	n1.Status = MemberStatusUp
	v.AddMember(n1)
	assert.Len(t, v.Members, 1)
	assert.Equal(t, 1, v.HealthyCount)
	assert.Equal(t, 1, v.QuorumSize)

	n2 := newNodeState("n2", "c1", "127.0.0.1:8002")
	n2.Status = MemberStatusUp
	v.AddMember(n2)
	assert.Len(t, v.Members, 2)
	assert.Equal(t, 2, v.HealthyCount)
	assert.Equal(t, 2, v.QuorumSize) // (2/2)+1 = 2

	v.RemoveMember("n1")
	assert.Len(t, v.Members, 1)
	assert.Equal(t, 1, v.HealthyCount)
	assert.Equal(t, 1, v.QuorumSize)
}

func TestClusterView_AddMember_IsNewerThan(t *testing.T) {
	deadline := time.Now().Add(testCaseTimeout)
	if d, ok := t.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	t.Cleanup(func() {
		if time.Now().After(deadline) {
			t.Error("test exceeded timeout")
		}
	})

	v := newClusterView()
	old := newNodeState("n1", "c1", "127.0.0.1:8001")
	old.Status = MemberStatusUp
	v.AddMember(old)

	newer := newNodeState("n1", "c1", "127.0.0.1:8001")
	newer.Generation = 2
	newer.Status = MemberStatusUp
	v.AddMember(newer)
	assert.Equal(t, 2, v.Members["n1"].Generation)

	older := newNodeState("n1", "c1", "127.0.0.1:8001")
	older.Generation = 1
	v.AddMember(older)
	assert.Equal(t, 2, v.Members["n1"].Generation)
}

func TestClusterView_Snapshot(t *testing.T) {
	deadline := time.Now().Add(testCaseTimeout)
	if d, ok := t.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	t.Cleanup(func() {
		if time.Now().After(deadline) {
			t.Error("test exceeded timeout")
		}
	})

	v := newClusterView()
	n := newNodeState("n1", "c1", "127.0.0.1:8001")
	n.Status = MemberStatusUp
	v.AddMember(n)

	snap := v.Snapshot()
	require.NotNil(t, snap)
	assert.Equal(t, v.ViewID, snap.ViewID)
	assert.Len(t, snap.Members, 1)
	assert.NotSame(t, v.Members["n1"], snap.Members["n1"])
	snap.Members["n1"].Generation = 99
	assert.Equal(t, 1, v.Members["n1"].Generation)
}

func TestClusterView_MergeFrom(t *testing.T) {
	deadline := time.Now().Add(testCaseTimeout)
	if d, ok := t.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	t.Cleanup(func() {
		if time.Now().After(deadline) {
			t.Error("test exceeded timeout")
		}
	})

	local := newClusterView()
	n1 := newNodeState("n1", "c1", "127.0.0.1:8001")
	n1.Status = MemberStatusUp
	local.AddMember(n1)

	other := newClusterView()
	n2 := newNodeState("n2", "c1", "127.0.0.1:8002")
	n2.Status = MemberStatusUp
	other.AddMember(n2)

	local.MergeFrom(other)
	assert.Len(t, local.Members, 2)
	assert.NotNil(t, local.Members["n1"])
	assert.NotNil(t, local.Members["n2"])
}

func TestClusterView_MergeFromWithOptions_VersionConcurrent(t *testing.T) {
	deadline := time.Now().Add(testCaseTimeout)
	if d, ok := t.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	t.Cleanup(func() {
		if time.Now().After(deadline) {
			t.Error("test exceeded timeout")
		}
	})

	// PreferLocal: 合并后保留本地 Epoch/Timestamp
	local := newClusterView()
	local.Epoch = 1
	local.Timestamp = 100
	local.VersionVector, _ = NewVersionVector().Increment("n1")
	other := newClusterView()
	other.Epoch = 2
	other.Timestamp = 200
	other.VersionVector, _ = NewVersionVector().Increment("n2")
	local.MergeFromWithOptions(other, MergeOptions{VersionConcurrentStrategy: 1})
	assert.Equal(t, int64(1), local.Epoch)
	assert.Equal(t, int64(100), local.Timestamp)

	// PreferRemote: 合并后采纳远端 Epoch/Timestamp（other 须含至少一名成员否则合并提前返回）
	local2 := newClusterView()
	local2.Epoch = 1
	local2.Timestamp = 100
	local2.VersionVector, _ = NewVersionVector().Increment("a")
	other2 := newClusterView()
	other2.Epoch = 2
	other2.Timestamp = 200
	other2.VersionVector, _ = NewVersionVector().Increment("b")
	n := newNodeState("b", "c1", "127.0.0.1:8002")
	other2.AddMember(n)
	local2.MergeFromWithOptions(other2, MergeOptions{VersionConcurrentStrategy: 2})
	assert.Equal(t, int64(2), local2.Epoch)
	assert.Equal(t, int64(200), local2.Timestamp)
}

func TestClusterView_MemberByAddress(t *testing.T) {
	v := newClusterView()
	n := newNodeState("n1", "c1", "127.0.0.1:8001")
	v.AddMember(n)
	assert.Same(t, v.Members["n1"], v.MemberByAddress("127.0.0.1:8001"))
	assert.Nil(t, v.MemberByAddress("127.0.0.1:9999"))
}
