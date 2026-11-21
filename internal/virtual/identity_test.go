package virtual_test

import (
	"testing"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/serialization"
	"github.com/kercylan98/vivid/internal/virtual"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestIdentity_CodecAndUnambiguousString(t *testing.T) {
	first := virtual.NewIdentity("a/b", "c")
	second := virtual.NewIdentity("a", "b/c")
	require.NotEqual(t, first.String(), second.String())
	for _, identity := range []*virtual.Identity{first, second, virtual.NewIdentity("类别%/", "名字%/"), virtual.NewIdentity("", "")} {
		writer := serialization.NewWriter()
		require.NoError(t, identity.Encode(writer, identity))
		var decoded virtual.Identity
		require.NoError(t, decoded.Decode(serialization.NewReader(writer.Bytes()), &decoded))
		require.True(t, identity.Equals(&decoded))
		// 截断不能将旧身份更新为只有部分字段的状态。
		previous := decoded.Clone()
		require.Error(t, decoded.Decode(serialization.NewReader(writer.Bytes()[:len(writer.Bytes())-1]), &decoded))
		require.True(t, decoded.Equals(previous))
	}
	var missing *virtual.Identity
	require.False(t, first.Equals(missing))
	codec := serialization.NewVividCodec(nil)
	require.NoError(t, codec.RegisterMessage("virtual", "Identity", new(virtual.Identity)))
	writer := serialization.NewWriter()
	require.NoError(t, codec.Encode(writer, first))
	decoded, err := codec.Decode(serialization.NewReader(writer.Bytes()))
	require.NoError(t, err)
	require.True(t, first.Equals(decoded.(vivid.ActorRef)))
}
