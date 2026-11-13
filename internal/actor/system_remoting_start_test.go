package actor_test

import (
	"crypto/tls"
	"net"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/actor"
	"github.com/kercylan98/vivid/internal/remoting"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystem_RemotingStartFailure(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer occupied.Close()
	for _, test := range []struct {
		name    string
		address string
		options []vivid.ActorSystemOption
	}{
		{"occupied", occupied.Addr().String(), nil},
		{"invalid_tls", clusterTestAddress(t), []vivid.ActorSystemOption{vivid.WithActorSystemRemotingOption(func(options *vivid.ActorSystemRemotingOptions) {
			options.TLSConfig = &tls.Config{}
		})}},
	} {
		t.Run(test.name, func(t *testing.T) {
			options := []vivid.ActorSystemOption{
				vivid.WithActorSystemRemoting(test.address),
				vivid.WithActorSystemStopTimeout(300 * time.Millisecond),
				vivid.WithActorSystemEnableMetrics(true),
				vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
			}
			system := actor.NewSystem(append(options, test.options...)...)
			err := system.Start()
			if err == nil {
				_ = system.Stop(time.Second)
			}
			assert.ErrorIs(t, err, vivid.ErrorActorSystemStartFailed)
			assert.ErrorIs(t, err, vivid.ErrorRemotingListen)
			assert.NotErrorIs(t, err, vivid.ErrorActorSystemStopFailed, "监听失败后的回滚不能再次停止超时")
			assert.ErrorIs(t, system.Stop(time.Second), vivid.ErrorActorSystemAlreadyStopped)
			assert.ErrorIs(t, system.Start(), vivid.ErrorActorSystemAlreadyStopped, "失败实例不可重新启动")
			if test.name == "occupied" {
				var listenError *net.OpError
				assert.ErrorAs(t, err, &listenError, "保留底层监听错误以便诊断")
			} else {
				listener, err := net.Listen("tcp", test.address)
				require.NoError(t, err, "无效 TLS 配置不得遗留监听端口")
				require.NoError(t, listener.Close())
			}
		})
	}
}

func TestSystem_RemotingStartReady(t *testing.T) {
	for range 16 {
		system := actor.NewSystem(
			vivid.WithActorSystemRemoting(clusterTestAddress(t)),
			vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
		)
		require.NoError(t, system.Start())
		func() {
			defer func() { require.NoError(t, system.Stop(time.Second)) }()
			// Start 成功后首次拨号即应连接到已绑定端口，不通过重试等待监听。
			conn, err := net.DialTimeout("tcp", system.Ref().GetAddress(), time.Second)
			require.NoError(t, err)
			defer conn.Close()
			require.NoError(t, conn.SetDeadline(time.Now().Add(time.Second)))
			require.NoError(t, (&remoting.Handshake{AdvertiseAddr: "127.0.0.1:1"}).Send(conn))
			require.NoError(t, new(remoting.Handshake).Wait(conn))
		}()
	}
}

func TestSystem_RemotingStartStopRebind(t *testing.T) {
	address := clusterTestAddress(t)
	for range 16 {
		system := actor.NewSystem(
			vivid.WithActorSystemRemoting(address),
			vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
		)
		require.NoError(t, system.Start())
		require.NoError(t, system.Stop(time.Second), "启动后立即停止不得等待尚未执行的启动回调")
		listener, err := net.Listen("tcp", address)
		require.NoError(t, err, "停止返回后监听端口应可重新使用")
		require.NoError(t, listener.Close())
	}
}

func TestSystem_RemotingStartRollback(t *testing.T) {
	address := clusterTestAddress(t)
	system := actor.NewSystem(
		vivid.WithActorSystemRemoting(address),
		vivid.WithActorSystemEnableMetrics(true),
		vivid.WithActorSystemStopTimeout(time.Second),
		vivid.WithActorSystemClusterOptions(vivid.WithActorSystemClusterName("vivid-test"), vivid.WithActorSystemClusterSeeds(address)),
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
	)
	// 只有自身种子在 Gossip 预启动阶段失败，此时 Remoting 已初始化。
	err := system.Start()
	require.ErrorIs(t, err, vivid.ErrorActorSystemStartFailed)
	require.ErrorIs(t, err, vivid.ErrorIllegalArgument)
	require.NotErrorIs(t, err, vivid.ErrorActorSystemStopFailed)
	listener, err := net.Listen("tcp", address)
	require.NoError(t, err, "后续组件启动失败时须释放已创建的监听端口")
	require.NoError(t, listener.Close())
}
