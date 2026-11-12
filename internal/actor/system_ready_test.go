package actor_test

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/actor"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pendingClusterSystem(t *testing.T, address string, options ...vivid.ActorSystemClusterOption) *actor.System {
	t.Helper()
	options = append([]vivid.ActorSystemClusterOption{
		vivid.WithActorSystemClusterLaunchDelay(0),
		vivid.WithActorSystemClusterGossipInterval(50 * time.Millisecond),
		vivid.WithActorSystemClusterGracefulShutdownTimeout(time.Second),
		vivid.WithActorSystemClusterJoinTimeout(200 * time.Millisecond),
	}, options...)
	system := actor.NewSystem(
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
		vivid.WithActorSystemRemoting(address),
		vivid.WithActorSystemClusterOptions(options...),
	)
	require.NoError(t, system.Start())
	t.Cleanup(func() {
		if err := system.Stop(3 * time.Second); err != nil {
			assert.ErrorIs(t, err, vivid.ErrorActorSystemAlreadyStopped)
		}
	})
	return system
}

func TestSystem_ClusterReady(t *testing.T) {
	t.Run("bootstrap", func(t *testing.T) {
		system := pendingClusterSystem(t, clusterTestAddress(t))
		result, err := system.ClusterReady(time.Second).Result()
		require.NoError(t, err)
		assert.True(t, result.Bootstrapped)
		assert.Empty(t, result.SeedAddress)
		snapshot, err := system.ClusterSnapshot(time.Second).Result()
		require.NoError(t, err)
		assert.Equal(t, snapshot.LocalID(), result.LocalID)
		member, found := snapshot.Member(result.LocalID)
		require.True(t, found)
		assert.Equal(t, vivid.ClusterMemberStatusUp, member.Status)
	})
	t.Run("join", func(t *testing.T) {
		address := clusterTestAddress(t)
		seed := startClusterTestSystem(t, address, 0)
		joiner := pendingClusterSystem(t, clusterTestAddress(t), vivid.WithActorSystemClusterSeeds(address))
		result, err := joiner.ClusterReady(time.Second).Result()
		require.NoError(t, err)
		assert.False(t, result.Bootstrapped)
		assert.Equal(t, address, result.SeedAddress)
		snapshot, err := joiner.ClusterSnapshot(time.Second).Result()
		require.NoError(t, err)
		assert.Equal(t, 2, snapshot.Len())
		seedResult, err := seed.ClusterReady(time.Second).Result()
		require.NoError(t, err)
		assert.NotEqual(t, seedResult.LocalID, result.LocalID)
	})
	t.Run("failure", func(t *testing.T) {
		system := pendingClusterSystem(t, clusterTestAddress(t), vivid.WithActorSystemClusterSeeds(clusterTestAddress(t)))
		_, err := system.ClusterReady(time.Second).Result()
		assert.ErrorIs(t, err, vivid.ErrorClusterJoinFailed)
		assert.ErrorIs(t, err, vivid.ErrorFutureTimeout)
		snapshot, err := system.ClusterSnapshot(time.Second).Result()
		require.NoError(t, err)
		local, _ := snapshot.Member(snapshot.LocalID())
		assert.Equal(t, vivid.ClusterMemberStatusJoining, local.Status)
		_, err = system.ClusterReady(time.Second).Result()
		assert.ErrorIs(t, err, vivid.ErrorClusterJoinFailed, "重复等待同一次初始结果，不暗中重新加入")
	})
	t.Run("explicit_bootstrap", func(t *testing.T) {
		system := pendingClusterSystem(t, clusterTestAddress(t),
			vivid.WithActorSystemClusterSeeds(clusterTestAddress(t)),
			vivid.WithActorSystemClusterBootstrap(true),
			vivid.WithActorSystemClusterJoinTimeout(time.Hour),
		)
		result, err := system.ClusterReady(time.Second).Result()
		require.NoError(t, err)
		assert.True(t, result.Bootstrapped)
	})
	t.Run("independent_waiters", func(t *testing.T) {
		system := pendingClusterSystem(t, clusterTestAddress(t), vivid.WithActorSystemClusterLaunchDelay(250*time.Millisecond))
		canceled := system.ClusterReady(time.Second)
		canceled.Close(context.Canceled)
		_, err := canceled.Result()
		assert.ErrorIs(t, err, context.Canceled)
		_, err = system.ClusterReady(time.Millisecond).Result()
		assert.ErrorIs(t, err, vivid.ErrorFutureTimeout)
		var wait sync.WaitGroup
		for range 20 {
			wait.Add(1)
			go func() {
				defer wait.Done()
				result, err := system.ClusterReady(time.Second).Result()
				assert.NoError(t, err)
				assert.True(t, result.Bootstrapped)
			}()
		}
		wait.Wait()
	})
	t.Run("next_seed", func(t *testing.T) {
		addresses := []string{clusterTestAddress(t), clusterTestAddress(t)}
		if addresses[0] > addresses[1] {
			addresses[0], addresses[1] = addresses[1], addresses[0]
		}
		seed := startClusterTestSystem(t, addresses[1], 0)
		joiner := pendingClusterSystem(t, clusterTestAddress(t),
			vivid.WithActorSystemClusterSeeds(" "+addresses[1]+" ", addresses[0], addresses[0], addresses[1]),
		)
		result, err := joiner.ClusterReady(2 * time.Second).Result()
		require.NoError(t, err, "较早种子失败后仍可加入后续种子")
		assert.Equal(t, addresses[1], result.SeedAddress)
		seedReady, err := seed.ClusterReady(time.Second).Result()
		require.NoError(t, err)
		assert.NotEqual(t, seedReady.LocalID, result.LocalID)
	})
	t.Run("pipe_to", func(t *testing.T) {
		system := pendingClusterSystem(t, clusterTestAddress(t), vivid.WithActorSystemClusterLaunchDelay(100*time.Millisecond))
		results := make(chan *vivid.PipeResult, 2)
		forwarded := 0
		_, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch message := ctx.Message().(type) {
			case *vivid.OnLaunch:
				ctx.System().ClusterReady(time.Second).PipeTo(ctx.Ref().ToActorRefs())
			case *vivid.PipeResult:
				results <- message
				forwarded++
				if forwarded == 1 {
					ctx.System().ClusterReady(time.Second).PipeTo(ctx.Ref().ToActorRefs())
				}
			}
		}))
		require.NoError(t, err)
		for range 2 {
			select {
			case result := <-results:
				require.NoError(t, result.Error)
				assert.True(t, result.Message.(vivid.ClusterJoinResult).Bootstrapped)
			case <-time.After(time.Second):
				t.Fatal("readiness was not forwarded")
			}
		}
	})
	for _, delay := range []bool{false, true} {
		name := "stop_during_join"
		if delay {
			name = "stop_during_launch_delay"
		}
		t.Run(name, func(t *testing.T) {
			options := []vivid.ActorSystemClusterOption{
				vivid.WithActorSystemClusterSeeds(clusterTestAddress(t)),
				vivid.WithActorSystemClusterJoinTimeout(time.Hour),
			}
			if delay {
				options = append(options, vivid.WithActorSystemClusterLaunchDelay(time.Hour))
			}
			system := pendingClusterSystem(t, clusterTestAddress(t), options...)
			waiting := system.ClusterReady(0)
			snapshot, err := system.ClusterSnapshot(time.Second).Result()
			require.NoError(t, err)
			local, _ := snapshot.Member(snapshot.LocalID())
			if delay {
				assert.Equal(t, vivid.ClusterMemberStatusNone, local.Status)
			} else {
				assert.Equal(t, vivid.ClusterMemberStatusJoining, local.Status)
			}
			require.NoError(t, system.Stop(time.Second), "停止不能等待启动延迟或种子超时")
			_, err = waiting.Result()
			assert.ErrorIs(t, err, vivid.ErrorActorSystemStopped)
			_, err = system.ClusterReady().Result()
			assert.ErrorIs(t, err, vivid.ErrorActorSystemStopped)
		})
	}
}

func TestSystem_ClusterReadyErrors(t *testing.T) {
	system := actor.NewSystem()
	_, err := system.ClusterReady().Result()
	assert.ErrorIs(t, err, vivid.ErrorActorSystemNotStarted)
	require.NoError(t, system.Start())
	_, err = system.ClusterReady().Result()
	assert.ErrorIs(t, err, vivid.ErrorClusterNotEnabled)
	require.NoError(t, system.Stop())
}

func TestSystem_ClusterInvalidOptions(t *testing.T) {
	for _, options := range []struct {
		name    string
		options []vivid.ActorSystemClusterOption
	}{
		{"negative_delay", []vivid.ActorSystemClusterOption{vivid.WithActorSystemClusterLaunchDelay(-1)}},
		{"zero_interval", []vivid.ActorSystemClusterOption{vivid.WithActorSystemClusterGossipInterval(0)}},
		{"negative_peers", []vivid.ActorSystemClusterOption{vivid.WithActorSystemClusterGossipPeersCount(-1)}},
		{"zero_shutdown", []vivid.ActorSystemClusterOption{vivid.WithActorSystemClusterGracefulShutdownTimeout(0)}},
		{"zero_join", []vivid.ActorSystemClusterOption{vivid.WithActorSystemClusterJoinTimeout(0)}},
		{"invalid_seed", []vivid.ActorSystemClusterOption{vivid.WithActorSystemClusterSeeds("node-without-port")}},
		{"wildcard_seed", []vivid.ActorSystemClusterOption{vivid.WithActorSystemClusterSeeds("0.0.0.0:8080")}},
	} {
		t.Run(options.name, func(t *testing.T) {
			address := clusterTestAddress(t)
			system := actor.NewSystem(vivid.WithActorSystemRemoting(address), vivid.WithActorSystemClusterOptions(options.options...))
			err := system.Start()
			assert.ErrorIs(t, err, vivid.ErrorActorSystemStartFailed)
			assert.ErrorIs(t, err, vivid.ErrorIllegalArgument)
			listener, err := net.Listen("tcp", address)
			require.NoError(t, err, "参数错误时不得遗留监听端口")
			require.NoError(t, listener.Close())
		})
	}
	address := clusterTestAddress(t)
	system := actor.NewSystem(vivid.WithActorSystemRemoting(address), vivid.WithActorSystemClusterOptions(vivid.WithActorSystemClusterSeeds(address)))
	err := system.Start()
	assert.ErrorIs(t, err, vivid.ErrorIllegalArgument, "只有自身种子时必须显式 Bootstrap")
}
