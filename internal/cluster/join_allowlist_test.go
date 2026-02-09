package cluster

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAllowJoinByDC(t *testing.T) {
	deadline := time.Now().Add(testCaseTimeout)
	if d, ok := t.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	t.Cleanup(func() {
		if time.Now().After(deadline) {
			t.Error("test exceeded timeout")
		}
	})

	assert.True(t, AllowJoinByDC("dc1", nil))
	assert.True(t, AllowJoinByDC("dc1", []string{}))
	assert.True(t, AllowJoinByDC("dc1", []string{"dc1"}))
	assert.False(t, AllowJoinByDC("dc1", []string{"dc2"}))
	assert.True(t, AllowJoinByDC("", []string{"_default"}))
	assert.True(t, AllowJoinByDC("_default", []string{"_default"}))
}

func TestAllowJoinByAddress_Exact(t *testing.T) {
	assert.True(t, AllowJoinByAddress("127.0.0.1:8001", []string{"127.0.0.1"}))
	assert.True(t, AllowJoinByAddress("127.0.0.1:8001", []string{"127.0.0.1:8001"}))
	assert.False(t, AllowJoinByAddress("127.0.0.1:8001", []string{"127.0.0.2"}))
}

func TestAllowJoinByAddress_CIDR(t *testing.T) {
	deadline := time.Now().Add(testCaseTimeout)
	if d, ok := t.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	t.Cleanup(func() {
		if time.Now().After(deadline) {
			t.Error("test exceeded timeout")
		}
	})

	assert.True(t, AllowJoinByAddress("192.168.1.10:8001", []string{"192.168.1.0/24"}))
	assert.False(t, AllowJoinByAddress("10.0.0.1:8001", []string{"192.168.1.0/24"}))
}

func TestAllowJoinByAddress_EmptyList(t *testing.T) {
	assert.True(t, AllowJoinByAddress("127.0.0.1:8001", nil))
	assert.True(t, AllowJoinByAddress("127.0.0.1:8001", []string{}))
}
