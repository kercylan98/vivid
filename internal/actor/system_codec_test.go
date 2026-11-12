package actor

import (
	"testing"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/serialization"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystem_CodecDecodeLimits(t *testing.T) {
	limits := vivid.DecodeLimits{MaxDepth: 8, MaxElements: 16, MaxAllocationBytes: 32}
	system := &System{options: vivid.NewActorSystemOptions(
		vivid.WithActorSystemRemotingOption(vivid.WithActorSystemRemotingDecodeLimits(limits)),
	)}
	require.NoError(t, newSystemChains(nil).initializeCodec(system).Run())
	assert.Equal(t, limits, system.codec.DecodeLimits())
	// 确认配置后仍能按原有规则路由框架消息。
	w := serialization.NewWriter()
	require.NoError(t, system.codec.Encode(w, &vivid.OnLaunch{}))
	r := serialization.NewReader(w.Bytes(), system.codec.DecodeLimits())
	message, err := system.codec.Decode(r)
	require.NoError(t, err)
	assert.IsType(t, &vivid.OnLaunch{}, message)
}
