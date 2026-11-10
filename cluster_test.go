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
}

func TestWithActorSystemClusterOptions_Defaults(t *testing.T) {
	opts := vivid.NewActorSystemOptions(vivid.WithActorSystemClusterOptions())

	require.NotNil(t, opts.ClusterOptions)
	assert.Empty(t, opts.ClusterOptions.Seeds)
	assert.GreaterOrEqual(t, opts.ClusterOptions.LaunchDelay, 50*time.Millisecond)
	assert.Less(t, opts.ClusterOptions.LaunchDelay, time.Second)
	assert.Equal(t, 200*time.Millisecond, opts.ClusterOptions.GossipInterval)
	assert.Equal(t, 5, opts.ClusterOptions.GossipPeersCount)
	assert.Equal(t, 10*time.Second, opts.ClusterOptions.GracefulShutdownTimeout)
	assert.Equal(t, 3*time.Second, opts.ClusterOptions.JoinTimeout)
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
