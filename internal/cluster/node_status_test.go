package cluster

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeState_IsNewerThan(t *testing.T) {
	deadline := time.Now().Add(testCaseTimeout)
	if d, ok := t.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	t.Cleanup(func() {
		if time.Now().After(deadline) {
			t.Error("test exceeded timeout")
		}
	})

	older := newNodeState("n1", "c1", "127.0.0.1:8001")
	newer := newNodeState("n1", "c1", "127.0.0.1:8001")
	newer.Generation = 2
	assert.True(t, newer.IsNewerThan(older))
	assert.False(t, older.IsNewerThan(newer))

	sameGenNewer := newNodeState("n1", "c1", "127.0.0.1:8001")
	sameGenNewer.Generation = 1
	sameGenNewer.LogicalClock = 2
	older.LogicalClock = 1
	assert.True(t, sameGenNewer.IsNewerThan(older))
}

func TestNodeState_Clone(t *testing.T) {
	n := newNodeState("n1", "c1", "127.0.0.1:8001")
	n.Labels[LabelDatacenter] = "dc1"
	c := n.Clone()
	require.NotNil(t, c)
	assert.Equal(t, n.ID, c.ID)
	require.NotNil(t, c.Labels)
	assert.Equal(t, "dc1", c.Labels[LabelDatacenter])
	c.Labels[LabelDatacenter] = "dc2"
	assert.Equal(t, "dc1", n.Labels[LabelDatacenter], "clone must not share map with original")
}

func TestNodeState_Datacenter_Rack_Region_Zone(t *testing.T) {
	n := newNodeState("n1", "c1", "127.0.0.1:8001")
	n.Labels[LabelDatacenter] = "dc1"
	n.Labels[LabelRack] = "r1"
	n.Labels[LabelRegion] = "reg1"
	n.Labels[LabelZone] = "z1"
	assert.Equal(t, "dc1", n.Datacenter())
	assert.Equal(t, "r1", n.Rack())
	assert.Equal(t, "reg1", n.Region())
	assert.Equal(t, "z1", n.Zone())
	assert.Empty(t, (*NodeState)(nil).Datacenter())
}

func TestMemberStatus_String(t *testing.T) {
	assert.Equal(t, "up", MemberStatusUp.String())
	assert.Equal(t, "joining", MemberStatusJoining.String())
	assert.Equal(t, "unknown", MemberStatus(99).String())
}

