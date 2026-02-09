package cluster

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestComputeJoinToken_VerifyJoinToken(t *testing.T) {
	deadline := time.Now().Add(testCaseTimeout)
	if d, ok := t.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	t.Cleanup(func() {
		if time.Now().After(deadline) {
			t.Error("test exceeded timeout")
		}
	})

	secret := "my-secret"
	state := newNodeState("n1", "cluster1", "127.0.0.1:8001")
	token := ComputeJoinToken(secret, state)
	assert.NotEmpty(t, token)
	assert.True(t, VerifyJoinToken(secret, token, state))
	assert.False(t, VerifyJoinToken(secret, token+"x", state))
	assert.False(t, VerifyJoinToken(secret, token, nil))
	assert.False(t, VerifyJoinToken("other", token, state))
}

func TestComputeJoinToken_EmptySecretOrState(t *testing.T) {
	state := newNodeState("n1", "c1", "127.0.0.1:8001")
	assert.Empty(t, ComputeJoinToken("", state))
	assert.Empty(t, ComputeJoinToken("s", nil))
}

func TestVerifyJoinToken_EmptySecret(t *testing.T) {
	assert.True(t, VerifyJoinToken("", "any", nil))
}

func TestComputeAdminToken_VerifyAdminToken(t *testing.T) {
	secret := "admin-secret"
	token := ComputeAdminToken(secret)
	assert.NotEmpty(t, token)
	assert.True(t, VerifyAdminToken(secret, token))
	assert.False(t, VerifyAdminToken(secret, token+"x"))
	assert.False(t, VerifyAdminToken("other", token))
}

func TestVerifyAdminToken_EmptySecret(t *testing.T) {
	assert.True(t, VerifyAdminToken("", "any"))
}

func TestComputeAdminToken_EmptySecret(t *testing.T) {
	assert.Empty(t, ComputeAdminToken(""))
}
