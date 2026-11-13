package gossip_test

import (
	"net"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/gossip"
	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/internal/gossip/gossipmessages"
	"github.com/kercylan98/vivid/pkg/bootstrap"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type droppedJoinReplyContext struct{ vivid.ActorContext }

func (*droppedJoinReplyContext) Reply(vivid.Message) {}

type lossyJoinActor struct {
	*gossip.SnapshotActor
	dropReply bool
	dropAll   bool
	attempts  chan int
	count     int
}

func (a *lossyJoinActor) OnReceive(ctx vivid.ActorContext) {
	if ping, ok := ctx.Message().(*gossipmessages.Ping); ok && ping.GossipType == gossipmessages.GossipTypeJoin {
		a.count++
		a.attempts <- a.count
		if a.count == 1 || a.dropAll {
			if a.dropReply {
				a.SnapshotActor.OnReceive(&droppedJoinReplyContext{ctx})
			}
			return
		}
	}
	a.SnapshotActor.OnReceive(ctx)
}

func TestActor_GossipJoinMessageLoss(t *testing.T) {
	for _, name := range []string{"request", "reply", "all_requests"} {
		t.Run(name, func(t *testing.T) {
			newSystem := func() vivid.PrimaryActorSystem {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)
				address := listener.Addr().String()
				require.NoError(t, listener.Close())
				system := bootstrap.NewActorSystem(
					vivid.WithActorSystemRemoting(address),
					vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
				)
				require.NoError(t, system.Start())
				t.Cleanup(func() { require.NoError(t, system.Stop(3*time.Second)) })
				return system
			}
			seed, joiner := newSystem(), newSystem()
			seedActor := &lossyJoinActor{
				SnapshotActor: gossip.NewSnapshotActor(vivid.NewActorSystemClusterOptions(
					vivid.WithActorSystemClusterName("vivid-test"),
					vivid.WithActorSystemClusterLaunchDelay(0),
					vivid.WithActorSystemClusterGracefulShutdownTimeout(time.Second),
				)),
				dropReply: name == "reply",
				dropAll:   name == "all_requests",
				attempts:  make(chan int, 3),
			}
			seedRef, err := seed.ActorOf(seedActor)
			require.NoError(t, err)
			_, err = seedActor.Readiness().Wait(seed, time.Second).Result()
			require.NoError(t, err)
			member := gossip.NewSnapshotActor(vivid.NewActorSystemClusterOptions(
				vivid.WithActorSystemClusterName("vivid-test"),
				vivid.WithActorSystemClusterSeeds(seedRef.GetAddress()),
				vivid.WithActorSystemClusterLaunchDelay(0),
				vivid.WithActorSystemClusterJoinTimeout(600*time.Millisecond),
				vivid.WithActorSystemClusterGracefulShutdownTimeout(time.Second),
			))
			_, err = joiner.ActorOf(member)
			require.NoError(t, err)
			// 调用等待与种子预算分开，避免繁忙环境下先收到调用方超时而掩盖加入结果。
			result, err := member.Readiness().Wait(joiner, 3*time.Second).Result()
			if name == "all_requests" {
				require.ErrorIs(t, err, vivid.ErrorClusterJoinFailed, "重发不能突破种子预算或无限重试")
				require.ErrorIs(t, err, vivid.ErrorFutureTimeout)
			} else {
				require.NoError(t, err, "首次请求或响应丢失后应在种子预算内重新请求")
				assert.Equal(t, seedRef.GetAddress(), result.SeedAddress)
			}
			assert.Equal(t, 1, <-seedActor.attempts)
			assert.Equal(t, 2, <-seedActor.attempts)
			if name == "all_requests" {
				assert.Equal(t, 3, <-seedActor.attempts)
			}
		})
	}
}

func TestActor_GossipJoinFailureDoesNotBootstrap(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())
	seed, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	seedAddress := seed.Addr().String()
	require.NoError(t, seed.Close())
	system := bootstrap.NewActorSystem(
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
		vivid.WithActorSystemRemoting(address),
	)
	require.NoError(t, system.Start())
	t.Cleanup(func() { require.NoError(t, system.Stop(3*time.Second)) })
	opts := vivid.NewActorSystemClusterOptions(
		vivid.WithActorSystemClusterName("vivid-test"),
		vivid.WithActorSystemClusterSeeds(seedAddress),
		vivid.WithActorSystemClusterLaunchDelay(0),
		vivid.WithActorSystemClusterGracefulShutdownTimeout(time.Second),
	)
	opts.JoinTimeout = 30 * time.Millisecond
	member := gossip.NewSnapshotActor(opts)
	ref, err := system.ActorOf(member)
	require.NoError(t, err)
	// 等待本轮所有种子尝试结束；配置种子不应隐式授予独立启动资格。
	_, err = member.Readiness().Wait(system, time.Second).Result()
	require.ErrorIs(t, err, vivid.ErrorClusterJoinFailed)
	result, err := system.Ask(ref, gossip.TestSnapshotRequest{}, time.Second).Result()
	require.NoError(t, err)
	assert.Equal(t, endpoint.StatusJoining, result.(gossip.SnapshotResponse).Status)
}
