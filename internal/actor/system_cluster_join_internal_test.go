package actor

import (
	"net"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/require"
)

func TestSystem_ClusterJoinRecoversWhenSeedStarts(t *testing.T) {
	newSystem := func(seeds ...string) *System {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		address := listener.Addr().String()
		require.NoError(t, listener.Close())
		system := NewSystem(
			vivid.WithActorSystemRemoting(address),
			vivid.WithActorSystemRemotingOption(vivid.WithActorSystemRemotingReconnectLimit(0)),
			vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
			vivid.WithActorSystemClusterOptions(
				vivid.WithActorSystemClusterName("vivid-test"),
				vivid.WithActorSystemClusterSeeds(seeds...),
				vivid.WithActorSystemClusterLaunchDelay(0),
				vivid.WithActorSystemClusterJoinTimeout(3*time.Second),
				vivid.WithActorSystemClusterGossipInterval(50*time.Millisecond),
				vivid.WithActorSystemClusterGracefulShutdownTimeout(time.Second),
			),
		)
		t.Cleanup(func() { require.NoError(t, system.Stop(3*time.Second)) })
		return system
	}
	seed := newSystem()
	seedAddress := seed.options.RemotingAdvertiseAddress
	joiner := newSystem(seedAddress)
	failed := make(chan error, 1)
	joiner.eventStream = &startupEventStream{EventStream: joiner.eventStream, observe: func(event vivid.Message) {
		if failure, ok := event.(ves.RemotingConnectionFailedEvent); ok && failure.Address == seedAddress {
			select {
			case failed <- failure.Error:
			default:
			}
		}
	}}
	require.NoError(t, joiner.Start())
	// 先确认真实拨号失败，再启动种子；关闭传输重连，恢复必须依赖新的加入请求。
	require.Error(t, receiveRetryResult(t, failed))
	snapshot, err := joiner.ClusterSnapshot(time.Second).Result()
	require.NoError(t, err)
	local, found := snapshot.Member(snapshot.LocalID())
	require.True(t, found)
	require.Equal(t, vivid.ClusterMemberStatusJoining, local.Status)

	require.NoError(t, seed.Start())
	_, err = seed.ClusterReady(time.Second).Result()
	require.NoError(t, err)
	result, err := joiner.ClusterReady(4 * time.Second).Result()
	require.NoError(t, err, "种子在加入预算内启动后应通过新的请求完成加入")
	require.Equal(t, seedAddress, result.SeedAddress)
	require.False(t, result.Bootstrapped)
	for _, system := range []*System{seed, joiner} {
		require.Eventually(t, func() bool {
			snapshot, err := system.ClusterSnapshot(time.Second).Result()
			if err != nil {
				return false
			}
			member, found := snapshot.Member(result.LocalID)
			return found && member.Status == vivid.ClusterMemberStatusUp && snapshot.Len() == 2
		}, 3*time.Second, 20*time.Millisecond)
	}
}
