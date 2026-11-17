package actor_test

import (
	"crypto/tls"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/actor"
	"github.com/kercylan98/vivid/internal/remoting"
	"github.com/kercylan98/vivid/internal/testkit"
	"github.com/kercylan98/vivid/pkg/metrics"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/require"
)

func observeRetiringEndpoints(t *testing.T, system *actor.System) (<-chan vivid.ActorRef, <-chan vivid.ActorRef, <-chan string) {
	t.Helper()
	ready := make(chan struct{})
	spawned, killed, closed := make(chan vivid.ActorRef, 64), make(chan vivid.ActorRef, 64), make(chan string, 64)
	_, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch message := ctx.Message().(type) {
		case *vivid.OnLaunch:
			ctx.EventStream().Subscribe(ctx, ves.ActorSpawnedEvent{})
			ctx.EventStream().Subscribe(ctx, ves.ActorKilledEvent{})
			ctx.EventStream().Subscribe(ctx, ves.RemotingConnectionClosedEvent{})
			close(ready)
		case ves.ActorSpawnedEvent:
			if message.Type.String() == "*remoting.endpoint" {
				spawned <- message.ActorRef
			}
		case ves.ActorKilledEvent:
			if message.Type.String() == "*remoting.endpoint" {
				killed <- message.ActorRef
			}
		case ves.RemotingConnectionClosedEvent:
			closed <- message.Address
		}
	}))
	require.NoError(t, err)
	receiveTellResult(t, ready)
	return spawned, killed, closed
}

func TestSystem_RemotingRetiresDisconnectedEndpoints(t *testing.T) {
	ca := testkit.NewTLSAuthority(t)
	for _, mode := range []string{"plaintext", "tls", "mtls"} {
		t.Run(mode, func(t *testing.T) {
			var serverConfig, clientConfig *tls.Config
			if mode != "plaintext" {
				serverConfig, clientConfig = ca.Config(t, mode == "mtls"), ca.Config(t, mode == "mtls")
			}
			server := startRemotingTLSSystem(t, serverConfig, vivid.WithActorSystemEnableMetrics(true))
			spawned, killed, closed := observeRetiringEndpoints(t, server)
			var previous vivid.ActorRef
			for index := range 32 {
				func() {
					dialer := &net.Dialer{Timeout: time.Second}
					var conn net.Conn
					var err error
					if clientConfig == nil {
						conn, err = dialer.Dial("tcp", server.Ref().GetAddress())
					} else {
						conn, err = tls.DialWithDialer(dialer, "tcp", server.Ref().GetAddress(), clientConfig)
					}
					require.NoError(t, err)
					defer conn.Close()
					require.NoError(t, conn.SetDeadline(time.Now().Add(time.Second)))
					// 前 24 次更换地址，后 8 次使用同一地址，验证重新接入会建立新的端点。
					address := fmt.Sprintf("127.0.0.1:%d", 20000+min(index, 23))
					require.NoError(t, (&remoting.Handshake{AdvertiseAddr: address}).Send(conn))
					require.NoError(t, new(remoting.Handshake).Wait(conn))
					ref := receiveTellResult(t, spawned)
					require.False(t, previous != nil && previous.Equals(ref))
					previous = ref
					_, err = server.FindActor(ref.String())
					require.NoError(t, err, "健康连接的空闲端点不能被回收")
					require.NoError(t, conn.Close())
					require.Equal(t, address, receiveTellResult(t, closed))
					require.True(t, ref.Equals(receiveTellResult(t, killed)))
					_, err = server.FindActor(ref.String())
					require.Error(t, err, "断开后无待处理工作的端点应自动移除")
					require.Zero(t, server.Metrics().Snapshot().GaugeOrZero(metrics.RemotingOutboundEndpointsGauge))
				}()
			}
			peer := startRemotingTLSSystem(t, clientConfig)
			ref, err := server.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
				if message, ok := ctx.Message().(vivid.ActorRef); ok {
					ctx.Reply(message)
				}
			}))
			require.NoError(t, err)
			message, err := peer.Ask(ref, peer.Ref(), time.Second).Result()
			require.NoError(t, err)
			require.True(t, peer.Ref().Equals(message.(vivid.ActorRef)))
		})
	}
}

func TestSystem_RemotingRetireAndSimultaneousReconnect(t *testing.T) {
	var systems []*actor.System
	var proxies []*delayedRemotingProxy
	var spawned, killed []<-chan vivid.ActorRef
	refs := make([]vivid.ActorRef, 2)
	for index := range 2 {
		backend := clusterTestAddress(t)
		proxy := newDelayedRemotingProxy(t, backend)
		t.Cleanup(proxy.Close)
		proxies = append(proxies, proxy)
		system := startRemotingTLSSystem(t, nil,
			vivid.WithActorSystemRemoting(backend, proxy.listener.Addr().String()),
			vivid.WithActorSystemRemotingOption(vivid.WithActorSystemRemotingReconnectLimit(0)),
			vivid.WithActorSystemEnableMetrics(true),
		)
		systems = append(systems, system)
		created, removed, _ := observeRetiringEndpoints(t, system)
		spawned, killed = append(spawned, created), append(killed, removed)
		ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			if message, ok := ctx.Message().(vivid.ActorRef); ok {
				ctx.Reply(message)
			}
		}))
		require.NoError(t, err)
		refs[index] = ref
	}
	for round := range 8 {
		for _, proxy := range proxies {
			proxy.SetAccepting(true)
		}
		start, completed := make(chan struct{}), make(chan error, 2)
		for index := range 2 {
			go func() {
				<-start
				message, err := systems[index].Ask(refs[1-index], systems[index].Ref(), time.Second).Result()
				if err == nil && !systems[index].Ref().Equals(message.(vivid.ActorRef)) {
					err = fmt.Errorf("第 %d 轮应答不匹配", round)
				}
				completed <- err
			}()
		}
		close(start)
		for range 2 {
			require.NoError(t, receiveTellResult(t, completed))
		}
		active := []vivid.ActorRef{receiveTellResult(t, spawned[0]), receiveTellResult(t, spawned[1])}
		// 先阻止所有方向的新接入，再断开已有链路，避免迟到的拨号重新建立连接。
		for _, proxy := range proxies {
			proxy.SetAccepting(false)
		}
		for _, proxy := range proxies {
			proxy.mu.Lock()
			for conn := range proxy.conns {
				_ = conn.Close()
			}
			proxy.mu.Unlock()
		}
		for index := range 2 {
			select {
			case removed := <-killed[index]:
				require.True(t, active[index].Equals(removed), "第 %d 轮节点 %d 的端点应被回收", round, index)
			case <-time.After(3 * time.Second):
				t.Fatalf("第 %d 轮节点 %d 等待端点 %s 回收超时", round, index, active[index])
			}
			_, err := systems[index].FindActor(active[index].String())
			require.ErrorIs(t, err, vivid.ErrorNotFound)
			require.Zero(t, systems[index].Metrics().Snapshot().GaugeOrZero(metrics.RemotingOutboundEndpointsGauge))
		}
	}
}
