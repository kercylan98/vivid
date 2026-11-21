package vivid_test

import (
	"strings"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithActorSystemClusterOptions_Initialize(t *testing.T) {
	opts := vivid.NewActorSystemOptions(
		vivid.WithActorSystemClusterOptions(
			vivid.WithActorSystemClusterName("vivid-test"),
			vivid.WithActorSystemClusterDataCenter("dc-a"),
			vivid.WithActorSystemClusterSeeds("127.0.0.1:7070"),
			vivid.WithActorSystemClusterLaunchDelay(100*time.Millisecond),
			vivid.WithActorSystemClusterGossipInterval(300*time.Millisecond),
			vivid.WithActorSystemClusterGossipPeersCount(3),
			vivid.WithActorSystemClusterMaxConcurrentProbes(7),
			vivid.WithActorSystemClusterGracefulShutdownTimeout(5*time.Second),
		),
	)

	require.NotNil(t, opts.ClusterOptions)
	assert.Equal(t, "vivid-test", opts.ClusterOptions.Name)
	assert.Equal(t, "dc-a", opts.ClusterOptions.DataCenter)
	assert.Equal(t, []string{"127.0.0.1:7070"}, opts.ClusterOptions.Seeds)
	assert.Equal(t, 100*time.Millisecond, opts.ClusterOptions.LaunchDelay)
	assert.Equal(t, 300*time.Millisecond, opts.ClusterOptions.GossipInterval)
	assert.Equal(t, 3, opts.ClusterOptions.GossipPeersCount)
	assert.Equal(t, 7, opts.ClusterOptions.MaxConcurrentProbes)
	assert.Equal(t, 5*time.Second, opts.ClusterOptions.GracefulShutdownTimeout)
	assert.Equal(t, 3*time.Second, opts.ClusterOptions.JoinTimeout)
	assert.Equal(t, 3*time.Second, opts.ClusterOptions.ProbeTimeout)
	assert.Equal(t, 10*time.Second, opts.ClusterOptions.ReachabilityTTL)
}

func TestWithActorSystemClusterOptions_Defaults(t *testing.T) {
	opts := vivid.NewActorSystemOptions(vivid.WithActorSystemClusterOptions())

	require.NotNil(t, opts.ClusterOptions)
	assert.Empty(t, opts.ClusterOptions.Name, "不使用公共默认名称，启用集群需明确命名")
	assert.ErrorIs(t, opts.ClusterOptions.Validate(), vivid.ErrorIllegalArgument)
	assert.Empty(t, opts.ClusterOptions.Seeds)
	assert.False(t, opts.ClusterOptions.Bootstrap)
	assert.GreaterOrEqual(t, opts.ClusterOptions.LaunchDelay, 50*time.Millisecond)
	assert.Less(t, opts.ClusterOptions.LaunchDelay, time.Second)
	assert.Equal(t, 200*time.Millisecond, opts.ClusterOptions.GossipInterval)
	assert.Equal(t, 5, opts.ClusterOptions.GossipPeersCount)
	assert.Equal(t, 5, opts.ClusterOptions.MaxConcurrentProbes)
	assert.Equal(t, 10*time.Second, opts.ClusterOptions.GracefulShutdownTimeout)
	assert.Equal(t, 3*time.Second, opts.ClusterOptions.JoinTimeout)
}

func TestActorSystemClusterOptions_Validate(t *testing.T) {
	var missing *vivid.ActorSystemClusterOptions
	assert.ErrorIs(t, missing.Validate(), vivid.ErrorIllegalArgument)
	options := vivid.NewActorSystemClusterOptions(
		vivid.WithActorSystemClusterName("vivid-test"),
		vivid.WithActorSystemClusterSeeds(" node.example:8080 ", "[::1]:8081"),
		vivid.WithActorSystemClusterJoinTimeout(5*time.Second),
		vivid.WithActorSystemClusterBootstrap(true),
	)
	assert.NoError(t, options.Validate())
	assert.Equal(t, 5*time.Second, options.JoinTimeout)
	assert.True(t, options.Bootstrap)
	for _, invalid := range []time.Duration{0, -time.Second} {
		options := vivid.NewActorSystemClusterOptions(vivid.WithActorSystemClusterName("vivid-test"), vivid.WithActorSystemClusterProbeTimeout(invalid))
		assert.ErrorIs(t, options.Validate(), vivid.ErrorIllegalArgument)
		options = vivid.NewActorSystemClusterOptions(vivid.WithActorSystemClusterName("vivid-test"), vivid.WithActorSystemClusterReachabilityTTL(invalid))
		assert.ErrorIs(t, options.Validate(), vivid.ErrorIllegalArgument)
	}
	valid := vivid.NewActorSystemClusterOptions(
		vivid.WithActorSystemClusterName("vivid-test"),
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
		vivid.WithActorSystemClusterOptions(
			vivid.WithActorSystemClusterName("vivid-test"),
			vivid.WithActorSystemClusterSeeds("127.0.0.1:7070"),
			vivid.WithActorSystemClusterJoinTimeout(5*time.Second),
			vivid.WithActorSystemClusterProbeTimeout(2*time.Second),
			vivid.WithActorSystemClusterReachabilityTTL(15*time.Second),
		),
	)
	clusterOptions := opts.ClusterOptions
	launchDelay := clusterOptions.LaunchDelay
	vivid.WithActorSystemClusterOptions(
		vivid.WithActorSystemClusterSeeds("127.0.0.1:8080"),
		vivid.WithActorSystemClusterGossipInterval(400*time.Millisecond),
	)(opts)

	assert.Same(t, clusterOptions, opts.ClusterOptions)
	assert.Equal(t, "vivid-test", opts.ClusterOptions.Name)
	assert.Equal(t, []string{"127.0.0.1:7070", "127.0.0.1:8080"}, opts.ClusterOptions.Seeds)
	assert.Equal(t, 400*time.Millisecond, opts.ClusterOptions.GossipInterval)
	assert.Equal(t, launchDelay, opts.ClusterOptions.LaunchDelay)
	assert.Equal(t, 5, opts.ClusterOptions.GossipPeersCount)
	assert.Equal(t, 10*time.Second, opts.ClusterOptions.GracefulShutdownTimeout)
	assert.Equal(t, 5*time.Second, opts.ClusterOptions.JoinTimeout)
	assert.Equal(t, 2*time.Second, opts.ClusterOptions.ProbeTimeout)
	assert.Equal(t, 15*time.Second, opts.ClusterOptions.ReachabilityTTL)
	assert.NoError(t, opts.ClusterOptions.Validate())

	before := *opts.ClusterOptions
	vivid.WithActorSystemClusterOptions()(opts)
	assert.Same(t, clusterOptions, opts.ClusterOptions)
	assert.Equal(t, before, *opts.ClusterOptions, "重复空调用不能丢弃名称、种子或重新随机生成启动延迟")
	vivid.WithActorSystemClusterOptions(vivid.WithActorSystemClusterName("vivid-next"))(opts)
	assert.Equal(t, "vivid-next", opts.ClusterOptions.Name, "指定字段按调用顺序覆盖")
}

func TestActorSystemClusterOptions_Name(t *testing.T) {
	for _, name := range []string{"", " ", "prod ", " prod", "prod test", "prod\n", "prod\x00", "prod\u3000", string([]byte{0xff}), strings.Repeat("x", 129)} {
		opts := vivid.NewActorSystemClusterOptions(vivid.WithActorSystemClusterName(name))
		assert.ErrorIs(t, opts.Validate(), vivid.ErrorIllegalArgument, "%q", name)
		assert.Equal(t, name, opts.Name, "校验不应暗中修正名称")
	}
	for _, name := range []string{"prod", "Prod", "prod.cn-1_a", "生产集群", strings.Repeat("x", 128)} {
		opts := vivid.NewActorSystemClusterOptions(vivid.WithActorSystemClusterName(name))
		assert.NoError(t, opts.Validate(), name)
		assert.Equal(t, name, opts.Name)
	}
}

func TestActorSystemClusterOptions_DataCenter(t *testing.T) {
	for _, dataCenter := range []string{" ", "dc-a ", " dc-a", "dc a", "dc\n", "dc\x00", "dc\u3000", string([]byte{0xff}), strings.Repeat("x", 129)} {
		opts := vivid.NewActorSystemClusterOptions(vivid.WithActorSystemClusterName("vivid-test"), vivid.WithActorSystemClusterDataCenter(dataCenter))
		assert.ErrorIs(t, opts.Validate(), vivid.ErrorIllegalArgument, "%q", dataCenter)
		assert.Equal(t, dataCenter, opts.DataCenter, "校验不应暗中修正标识")
	}
	for _, dataCenter := range []string{"", "dc-a", "DC-a", "cn-east_1.az-a", "上海机房", strings.Repeat("x", 128)} {
		opts := vivid.NewActorSystemOptions(vivid.WithActorSystemClusterOptions(vivid.WithActorSystemClusterName("vivid-test"), vivid.WithActorSystemClusterDataCenter(dataCenter)))
		require.NoError(t, opts.ClusterOptions.Validate(), "%q", dataCenter)
		vivid.WithActorSystemClusterOptions(vivid.WithActorSystemClusterSeeds("node.example:8080"))(opts)
		assert.Equal(t, dataCenter, opts.ClusterOptions.DataCenter, "增量调用保留机房声明")
		vivid.WithActorSystemClusterOptions(vivid.WithActorSystemClusterDataCenter("dc-b"))(opts)
		assert.Equal(t, "dc-b", opts.ClusterOptions.DataCenter)
		vivid.WithActorSystemClusterOptions(vivid.WithActorSystemClusterDataCenter(""))(opts)
		assert.Empty(t, opts.ClusterOptions.DataCenter, "启动前可显式清除声明")
	}
}

func TestActorSystemClusterOptions_ProbeConcurrency(t *testing.T) {
	for _, count := range []int{0, -1} {
		opts := vivid.NewActorSystemClusterOptions(vivid.WithActorSystemClusterName("vivid-test"), vivid.WithActorSystemClusterMaxConcurrentProbes(count))
		assert.ErrorIs(t, opts.Validate(), vivid.ErrorIllegalArgument)
	}
	for _, count := range []int{1, 3, 20} {
		opts := vivid.NewActorSystemOptions(vivid.WithActorSystemClusterOptions(vivid.WithActorSystemClusterName("vivid-test"), vivid.WithActorSystemClusterGossipPeersCount(count)))
		assert.Equal(t, 5, opts.ClusterOptions.MaxConcurrentProbes, "每轮发送数量不应改变默认观察并发")
		vivid.WithActorSystemClusterOptions(vivid.WithActorSystemClusterMaxConcurrentProbes(7))(opts)
		assert.Equal(t, count, opts.ClusterOptions.GossipPeersCount, "观察并发不应改变每轮发送数量")
		vivid.WithActorSystemClusterOptions(vivid.WithActorSystemClusterGossipPeersCount(2))(opts)
		assert.Equal(t, 7, opts.ClusterOptions.MaxConcurrentProbes, "增量修改发送数量时保留观察并发")
		vivid.WithActorSystemClusterOptions(vivid.WithActorSystemClusterMaxConcurrentProbes(1))(opts)
		assert.Equal(t, 1, opts.ClusterOptions.MaxConcurrentProbes)
		assert.Equal(t, 2, opts.ClusterOptions.GossipPeersCount)
		assert.NoError(t, opts.ClusterOptions.Validate(), "观察并发可小于每轮发送数量")
		before := *opts.ClusterOptions
		vivid.WithActorSystemClusterOptions()(opts)
		assert.Equal(t, before, *opts.ClusterOptions)
	}
}
