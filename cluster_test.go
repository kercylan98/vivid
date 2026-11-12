package vivid_test

import (
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithActorSystemClusterOptions_Initialize(t *testing.T) {
	opts := vivid.NewActorSystemOptions(
		vivid.WithActorSystemClusterOptions(
			vivid.WithActorSystemClusterSeeds("127.0.0.1:7070"),
			vivid.WithActorSystemClusterLaunchDelay(100*time.Millisecond),
			vivid.WithActorSystemClusterGossipInterval(300*time.Millisecond),
			vivid.WithActorSystemClusterGossipPeersCount(3),
			vivid.WithActorSystemClusterGracefulShutdownTimeout(5*time.Second),
		),
	)

	require.NotNil(t, opts.ClusterOptions)
	assert.Equal(t, []string{"127.0.0.1:7070"}, opts.ClusterOptions.Seeds)
	assert.Equal(t, 100*time.Millisecond, opts.ClusterOptions.LaunchDelay)
	assert.Equal(t, 300*time.Millisecond, opts.ClusterOptions.GossipInterval)
	assert.Equal(t, 3, opts.ClusterOptions.GossipPeersCount)
	assert.Equal(t, 5*time.Second, opts.ClusterOptions.GracefulShutdownTimeout)
	assert.Equal(t, 3*time.Second, opts.ClusterOptions.JoinTimeout)
	assert.Equal(t, 3*time.Second, opts.ClusterOptions.ProbeTimeout)
	assert.Equal(t, 10*time.Second, opts.ClusterOptions.ReachabilityTTL)
}

func TestWithActorSystemClusterOptions_Defaults(t *testing.T) {
	opts := vivid.NewActorSystemOptions(vivid.WithActorSystemClusterOptions())

	require.NotNil(t, opts.ClusterOptions)
	assert.Empty(t, opts.ClusterOptions.Seeds)
	assert.False(t, opts.ClusterOptions.Bootstrap)
	assert.GreaterOrEqual(t, opts.ClusterOptions.LaunchDelay, 50*time.Millisecond)
	assert.Less(t, opts.ClusterOptions.LaunchDelay, time.Second)
	assert.Equal(t, 200*time.Millisecond, opts.ClusterOptions.GossipInterval)
	assert.Equal(t, 5, opts.ClusterOptions.GossipPeersCount)
	assert.Equal(t, 10*time.Second, opts.ClusterOptions.GracefulShutdownTimeout)
	assert.Equal(t, 3*time.Second, opts.ClusterOptions.JoinTimeout)
}

func TestActorSystemClusterOptions_Validate(t *testing.T) {
	var missing *vivid.ActorSystemClusterOptions
	assert.ErrorIs(t, missing.Validate(), vivid.ErrorIllegalArgument)
	options := vivid.NewActorSystemClusterOptions(
		vivid.WithActorSystemClusterSeeds(" node.example:8080 ", "[::1]:8081"),
		vivid.WithActorSystemClusterJoinTimeout(5*time.Second),
		vivid.WithActorSystemClusterBootstrap(true),
	)
	assert.NoError(t, options.Validate())
	assert.Equal(t, 5*time.Second, options.JoinTimeout)
	assert.True(t, options.Bootstrap)
	for _, invalid := range []time.Duration{0, -time.Second} {
		options := vivid.NewActorSystemClusterOptions(vivid.WithActorSystemClusterProbeTimeout(invalid))
		assert.ErrorIs(t, options.Validate(), vivid.ErrorIllegalArgument)
		options = vivid.NewActorSystemClusterOptions(vivid.WithActorSystemClusterReachabilityTTL(invalid))
		assert.ErrorIs(t, options.Validate(), vivid.ErrorIllegalArgument)
	}
	valid := vivid.NewActorSystemClusterOptions(
		vivid.WithActorSystemClusterProbeTimeout(2*time.Second),
		vivid.WithActorSystemClusterReachabilityTTL(15*time.Second),
	)
	assert.NoError(t, valid.Validate())
	assert.Equal(t, 2*time.Second, valid.ProbeTimeout)
	assert.Equal(t, 15*time.Second, valid.ReachabilityTTL)
	for _, address := range []string{"", "node", "node:0", "node:65536", "[::]:8080", "0.0.0.0:8080", "127.0.0.1:8080:/@gossip"} {
		options.Seeds = []string{address}
		assert.ErrorIs(t, options.Validate(), vivid.ErrorIllegalArgument, address)
	}
}

func TestWithActorSystemClusterOptions_Append(t *testing.T) {
	opts := vivid.NewActorSystemOptions(
		vivid.WithActorSystemClusterOptions(vivid.WithActorSystemClusterSeeds("127.0.0.1:7070")),
	)
	clusterOptions := opts.ClusterOptions
	launchDelay := clusterOptions.LaunchDelay
	vivid.WithActorSystemClusterOptions(
		vivid.WithActorSystemClusterSeeds("127.0.0.1:8080"),
		vivid.WithActorSystemClusterGossipInterval(400*time.Millisecond),
	)(opts)

	assert.Same(t, clusterOptions, opts.ClusterOptions)
	assert.Equal(t, []string{"127.0.0.1:7070", "127.0.0.1:8080"}, opts.ClusterOptions.Seeds)
	assert.Equal(t, 400*time.Millisecond, opts.ClusterOptions.GossipInterval)
	assert.Equal(t, launchDelay, opts.ClusterOptions.LaunchDelay)
	assert.Equal(t, 5, opts.ClusterOptions.GossipPeersCount)
	assert.Equal(t, 10*time.Second, opts.ClusterOptions.GracefulShutdownTimeout)
	assert.Equal(t, 3*time.Second, opts.ClusterOptions.JoinTimeout)
}
