package actor_test

import (
	"net"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/actor"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystem_RemotingAdvertiseInvalid(t *testing.T) {
	for _, cluster := range []bool{false, true} {
		for _, advertised := range []string{"", "node.example", "127.0.0.1", "0.0.0.0:8080", "[::]:8080", "[0:0:0:0:0:0:0:0]:8080", "[::ffff:0.0.0.0]:8080", ":8080", "node:0", "node:65536", "node:+80", "bad host:80"} {
			t.Run(map[bool]string{true: "cluster", false: "remoting"}[cluster]+"/"+advertised, func(t *testing.T) {
				address := clusterTestAddress(t)
				options := []vivid.ActorSystemOption{
					vivid.WithActorSystemRemoting(address, advertised),
					vivid.WithActorSystemStopTimeout(time.Second),
					vivid.WithActorSystemEnableMetrics(true),
					vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
				}
				if cluster {
					options = append(options, vivid.WithActorSystemClusterOptions(vivid.WithActorSystemClusterName("vivid-test")))
				}
				var system *actor.System
				require.NotPanics(t, func() { system = actor.NewSystem(options...) }, "配置错误由 Start 返回")
				err := system.Start()
				if err == nil {
					_ = system.Stop(time.Second)
				}
				assert.ErrorIs(t, err, vivid.ErrorActorSystemStartFailed)
				assert.ErrorIs(t, err, vivid.ErrorIllegalArgument)
				assert.ErrorContains(t, err, "advertise address")
				assert.NotErrorIs(t, err, vivid.ErrorActorSystemStopFailed)
				assert.ErrorIs(t, system.Start(), vivid.ErrorActorSystemAlreadyStopped)
				listener, err := net.Listen("tcp", address)
				require.NoError(t, err, "广告地址错误不得保留监听端口")
				require.NoError(t, listener.Close())
			})
		}
	}
	address := clusterTestAddress(t)
	_, port, err := net.SplitHostPort(address)
	require.NoError(t, err)
	for _, host := range []string{"", "0.0.0.0", "::"} {
		system := actor.NewSystem(vivid.WithActorSystemRemoting(net.JoinHostPort(host, port)))
		assert.ErrorIs(t, system.Start(), vivid.ErrorIllegalArgument, "通配监听的默认广告地址也必须拒绝")
	}
	system := actor.NewSystem(vivid.WithActorSystemRemoting("", address))
	assert.ErrorIs(t, system.Start(), vivid.ErrorIllegalArgument, "广告地址不能代替监听配置")
}

func TestSystem_RemotingAdvertiseCommunication(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "localhost", "::1"} {
		t.Run(host, func(t *testing.T) {
			bindHost := "0.0.0.0"
			address := clusterTestAddress(t)
			if host == "::1" {
				listener, err := net.Listen("tcp6", "[::1]:0")
				if err != nil {
					t.Skip("当前环境不支持 IPv6 监听")
				}
				address, bindHost = listener.Addr().String(), "::"
				require.NoError(t, listener.Close())
			}
			_, port, err := net.SplitHostPort(address)
			require.NoError(t, err)
			advertised := net.JoinHostPort(host, port)
			server := actor.NewSystem(
				vivid.WithActorSystemRemoting(net.JoinHostPort(bindHost, port), " "+advertised+" "),
				vivid.WithActorSystemClusterOptions(
					vivid.WithActorSystemClusterName("vivid-test"),
					vivid.WithActorSystemClusterLaunchDelay(0),
					vivid.WithActorSystemClusterGossipInterval(time.Hour),
					vivid.WithActorSystemClusterGracefulShutdownTimeout(50*time.Millisecond),
				),
				vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
			)
			require.NoError(t, server.Start())
			defer func() { require.NoError(t, server.Stop(3*time.Second)) }()
			result, err := server.ClusterReady(time.Second).Result()
			require.NoError(t, err)
			assert.True(t, result.Bootstrapped)
			assert.Equal(t, advertised, server.Ref().GetAddress())
			client := actor.NewSystem(
				vivid.WithActorSystemRemoting(clusterTestAddress(t)),
				vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
			)
			require.NoError(t, client.Start())
			defer func() { require.NoError(t, client.Stop(3*time.Second)) }()
			ref, err := server.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
				if message, ok := ctx.Message().(vivid.ActorRef); ok {
					ctx.Reply(message)
				}
			}))
			require.NoError(t, err)
			message, err := client.Ask(ref, client.Ref(), 2*time.Second).Result()
			require.NoError(t, err, "经广告地址完成真实 TCP 请求与应答")
			reply, ok := message.(vivid.ActorRef)
			require.True(t, ok)
			assert.True(t, client.Ref().Equals(reply))
		})
	}
}
