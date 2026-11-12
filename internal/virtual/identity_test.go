package virtual_test

import (
	"testing"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/virtual"
	"github.com/stretchr/testify/assert"
)

func TestIdentity_Clone(t *testing.T) {
	identity := virtual.NewIdentity("test", "123")
	clone := identity.Clone()
	assert.True(t, identity.Equals(clone))
}

func TestIdentity_Equals(t *testing.T) {
	var cases = []struct {
		name      string
		identity1 *virtual.Identity
		identity2 *virtual.Identity
		want      bool
	}{
		{name: "same", identity1: virtual.NewIdentity("test", "123"), identity2: virtual.NewIdentity("test", "123"), want: true},
		{name: "different", identity1: virtual.NewIdentity("test", "123"), identity2: virtual.NewIdentity("test", "456"), want: false},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.identity1.Equals(tt.identity2))
		})
	}
}

func TestIdentity_GetAddress(t *testing.T) {
	identity := virtual.NewIdentity("test", "123")
	assert.Equal(t, "virtual:test", identity.GetAddress())
}

func TestIdentity_GetPath(t *testing.T) {
	identity := virtual.NewIdentity("test", "123")
	assert.Equal(t, "123", identity.GetPath())
}

func TestIdentity_String(t *testing.T) {
	identity := virtual.NewIdentity("test", "123")
	assert.Equal(t, "virtual:test/123", identity.String())
}

func TestIdentity_ToActorRefs(t *testing.T) {
	identity := virtual.NewIdentity("test", "123")
	assert.Equal(t, vivid.ActorRefs{identity}, identity.ToActorRefs())
}

func TestIdentity_IsVirtual(t *testing.T) {
	identity := virtual.NewIdentity("test", "123")
	assert.True(t, identity.IsVirtual())
}
