package cluster

import (
	"testing"
	"time"

	"github.com/kercylan98/vivid/internal/messages"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionVector_Compare(t *testing.T) {
	deadline := time.Now().Add(testCaseTimeout)
	if d, ok := t.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	t.Cleanup(func() {
		if time.Now().After(deadline) {
			t.Error("test exceeded timeout")
		}
	})

	empty := NewVersionVector()
	assert.Equal(t, VersionEqual, empty.Compare(NewVersionVector()))

	v1, _ := empty.Increment("a")
	v2, _ := v1.Increment("a")
	assert.Equal(t, VersionBefore, v1.Compare(v2))
	assert.Equal(t, VersionAfter, v2.Compare(v1))

	va, _ := empty.Increment("a")
	vb, _ := empty.Increment("b")
	assert.Equal(t, VersionConcurrent, va.Compare(vb))
	assert.Equal(t, VersionConcurrent, vb.Compare(va))
}

func TestVersionVector_Merge(t *testing.T) {
	deadline := time.Now().Add(testCaseTimeout)
	if d, ok := t.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	t.Cleanup(func() {
		if time.Now().After(deadline) {
			t.Error("test exceeded timeout")
		}
	})

	va, _ := NewVersionVector().Increment("a")
	va, _ = va.Increment("a")
	vb, _ := NewVersionVector().Increment("b")
	merged := va.Merge(vb)
	assert.Equal(t, uint64(2), merged.Get("a"))
	assert.Equal(t, uint64(1), merged.Get("b"))
}

func TestVersionVector_Increment_Prune(t *testing.T) {
	deadline := time.Now().Add(testCaseTimeout)
	if d, ok := t.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	t.Cleanup(func() {
		if time.Now().After(deadline) {
			t.Error("test exceeded timeout")
		}
	})

	v, err := NewVersionVector().Increment("n1")
	require.NoError(t, err)
	v, err = v.Increment("n2")
	require.NoError(t, err)
	pruned := v.PruneWithMax([]string{"n1"}, 10)
	assert.Equal(t, uint64(1), pruned.Get("n1"))
	assert.Equal(t, uint64(0), pruned.Get("n2"))
	assert.Equal(t, 1, pruned.Size())
}

func TestVersionVector_ReadWrite(t *testing.T) {
	deadline := time.Now().Add(testCaseTimeout)
	if d, ok := t.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	t.Cleanup(func() {
		if time.Now().After(deadline) {
			t.Error("test exceeded timeout")
		}
	})

	v, _ := NewVersionVector().Increment("a")
	v, _ = v.Increment("b")
	w := messages.NewWriter()
	err := WriteVersionVector(w, v)
	require.NoError(t, err)
	data := w.Bytes()
	require.NotEmpty(t, data)

	r := messages.NewReader(data)
	dec, err := ReadVersionVector(r)
	require.NoError(t, err)
	assert.True(t, v.Equal(dec))
}

func TestVersionVector_ValidateNodeAddress(t *testing.T) {
	_, err := NewVersionVector().Increment("")
	assert.Error(t, err)
}
