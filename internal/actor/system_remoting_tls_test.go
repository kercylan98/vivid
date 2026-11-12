package actor_test

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/actor"
	"github.com/kercylan98/vivid/internal/testkit"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/require"
)

func startRemotingTLSSystem(t testing.TB, config *tls.Config, options ...vivid.ActorSystemOption) *actor.System {
	t.Helper()
	defaults := []vivid.ActorSystemOption{
		vivid.WithActorSystemRemoting(clusterTestAddress(t)),
		vivid.WithActorSystemRemotingOption(
			vivid.WithActorSystemRemotingTLSConfig(config),
		),
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
	}
	system := actor.NewSystem(append(defaults, options...)...)
	require.NoError(t, system.Start())
	t.Cleanup(func() {
		err := system.Stop(3 * time.Second)
		require.True(t, err == nil || errors.Is(err, vivid.ErrorActorSystemAlreadyStopped), "%v", err)
	})
	return system
}

func BenchmarkSystem_RemotingTLSRoundTrip(b *testing.B) {
	ca := testkit.NewTLSAuthority(b)
	for _, mode := range []string{"plaintext", "tls", "mtls"} {
		b.Run(mode, func(b *testing.B) {
			var configs [2]*tls.Config
			if mode != "plaintext" {
				for i := range configs {
					configs[i] = ca.Config(b, mode == "mtls")
				}
			}
			server := startRemotingTLSSystem(b, configs[0])
			client := startRemotingTLSSystem(b, configs[1])
			ref, err := server.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
				if message, ok := ctx.Message().(vivid.ActorRef); ok {
					ctx.Reply(message)
				}
			}))
			require.NoError(b, err)
			message := client.Ref()
			// 先建立连接并完成证书校验，计时只包含复用连接后的完整 Ask/Reply。
			_, err = client.Ask(ref, message, time.Second).Result()
			require.NoError(b, err)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				result, err := client.Ask(ref, message, time.Second).Result()
				if err != nil || !message.Equals(result.(vivid.ActorRef)) {
					b.Fatalf("远程应答不匹配: %v", err)
				}
			}
			b.StopTimer()
		})
	}
}

func TestSystem_RemotingTLSExchange(t *testing.T) {
	ca := testkit.NewTLSAuthority(t)
	for _, mutual := range []bool{false, true} {
		for _, first := range []int{0, 1, 2} {
			t.Run("mutual="+strconv.FormatBool(mutual)+"/first="+strconv.Itoa(first), func(t *testing.T) {
				configs := []*tls.Config{ca.Config(t, mutual), ca.Config(t, mutual)}
				var systems []*actor.System
				for _, config := range configs {
					systems = append(systems, startRemotingTLSSystem(t, config, vivid.WithActorSystemRemotingOption(
						vivid.WithActorSystemRemotingReconnectLimit(0),
					)))
				}
				refs := make([]vivid.ActorRef, 2)
				for i, system := range systems {
					ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
						if message, ok := ctx.Message().(vivid.ActorRef); ok {
							ctx.Reply(message)
						}
					}))
					require.NoError(t, err)
					refs[i] = ref
				}
				ask := func(sender int) error {
					message := systems[sender].Ref()
					result, err := systems[sender].Ask(refs[1-sender], message, time.Second).Result()
					if err == nil && !message.Equals(result.(vivid.ActorRef)) {
						return errors.New("TLS reply mismatch")
					}
					return err
				}
				if first == 2 {
					start := make(chan struct{})
					results := make(chan error, 2)
					for sender := range 2 {
						go func() {
							<-start
							results <- ask(sender)
						}()
					}
					close(start)
					for range 2 {
						require.NoError(t, receiveTellResult(t, results))
					}
				} else {
					require.NoError(t, ask(first))
					require.NoError(t, ask(1-first))
				}
				for range 8 {
					require.NoError(t, ask(0))
					require.NoError(t, ask(1))
				}
				require.Empty(t, configs[0].ServerName, "不能把目标地址写入共享 TLS 配置")
				require.Empty(t, configs[1].ServerName)
			})
		}
	}
}

func TestSystem_RemotingTLSRejectsPeer(t *testing.T) {
	ca := testkit.NewTLSAuthority(t)
	for _, name := range []string{"unknown_ca", "wrong_name", "missing_client_certificate", "untrusted_client", "plaintext_client", "plaintext_server"} {
		t.Run(name, func(t *testing.T) {
			serverConfig, clientConfig := ca.Config(t, true), ca.Config(t, true)
			switch name {
			case "unknown_ca":
				clientConfig.RootCAs = x509.NewCertPool()
			case "wrong_name":
				clientConfig.ServerName = "other.invalid"
			case "missing_client_certificate":
				clientConfig.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) { return &tls.Certificate{}, nil }
			case "untrusted_client":
				other := testkit.NewTLSAuthority(t).Config(t, true)
				clientConfig.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) { return &other.Certificates[0], nil }
			case "plaintext_client":
				clientConfig = nil
			case "plaintext_server":
				serverConfig = nil
			}
			server := startRemotingTLSSystem(t, serverConfig)
			client := startRemotingTLSSystem(t, clientConfig, vivid.WithActorSystemRemotingOption(
				vivid.WithActorSystemRemotingReconnectLimit(0),
			))
			delivered := make(chan struct{}, 1)
			ref, err := server.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
				if _, ok := ctx.Message().(vivid.ActorRef); ok {
					delivered <- struct{}{}
				}
			}))
			require.NoError(t, err)
			ready := make(chan struct{})
			failed := make(chan error, 4)
			_, err = client.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
				switch message := ctx.Message().(type) {
				case *vivid.OnLaunch:
					ctx.EventStream().Subscribe(ctx, ves.RemotingConnectionFailedEvent{})
					close(ready)
				case ves.RemotingConnectionFailedEvent:
					if message.Address == ref.GetAddress() {
						failed <- message.Error
					}
				}
			}))
			require.NoError(t, err)
			receiveTellResult(t, ready)
			client.Tell(ref, client.Ref())
			err = receiveTellResult(t, failed)
			require.Error(t, err)
			if name == "unknown_ca" {
				var unknown x509.UnknownAuthorityError
				require.ErrorAs(t, err, &unknown)
			}
			if name == "wrong_name" {
				var hostname x509.HostnameError
				require.ErrorAs(t, err, &hostname)
			}
			select {
			case <-delivered:
				t.Fatal("未通过 TLS 校验的消息被投递")
			default:
			}
		})
	}
}

func TestSystem_RemotingTLSStopHandshake(t *testing.T) {
	ca := testkit.NewTLSAuthority(t)
	t.Run("inbound", func(t *testing.T) {
		system := startRemotingTLSSystem(t, ca.Config(t, true))
		conn, err := net.DialTimeout("tcp", system.Ref().GetAddress(), time.Second)
		require.NoError(t, err)
		defer conn.Close()
		_, err = conn.Write([]byte{22})
		require.NoError(t, err)
		require.NoError(t, system.Stop(time.Second))
		require.NoError(t, conn.SetReadDeadline(time.Now().Add(time.Second)))
		_, err = io.Copy(io.Discard, conn)
		// 未读完的 TLS 请求可能使 TCP 返回 reset；不能超时等待仍未关闭的连接。
		if err != nil {
			var networkError net.Error
			require.ErrorAs(t, err, &networkError)
			require.False(t, networkError.Timeout(), "停止必须解除未完成的 TLS 入站握手")
		}
	})
	t.Run("outbound", func(t *testing.T) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		defer listener.Close()
		accepted := make(chan net.Conn, 1)
		go func() {
			conn, err := listener.Accept()
			if err == nil {
				accepted <- conn
			}
		}()
		system := startRemotingTLSSystem(t, ca.Config(t, true))
		ref, err := system.CreateRef(listener.Addr().String(), "/silent")
		require.NoError(t, err)
		system.Tell(ref, system.Ref())
		conn := receiveTellResult(t, accepted)
		defer conn.Close()
		require.NoError(t, conn.SetReadDeadline(time.Now().Add(time.Second)))
		header := make([]byte, 5)
		_, err = io.ReadFull(conn, header)
		require.NoError(t, err)
		require.Equal(t, byte(22), header[0], "出站首先发送 TLS 握手，不能发送明文 Vivid 帧")
		require.NoError(t, system.Stop(time.Second))
		_, err = io.Copy(io.Discard, conn)
		require.NoError(t, err, "停止必须关闭未完成 TLS 握手的出站 socket")
	})
}

// remotingTLSProxy 只转发 TCP 字节，不解密或识别 TLS、Vivid 帧。
type remotingTLSProxy struct {
	listener net.Listener
	backend  string
	mu       sync.Mutex
	conns    map[net.Conn]struct{}
	blocked  bool
	closed   bool
	rejected int
	wg       sync.WaitGroup
}

func newRemotingTLSProxy(t *testing.T, backend string) *remotingTLSProxy {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	p := &remotingTLSProxy{listener: listener, backend: backend, conns: make(map[net.Conn]struct{})}
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			p.mu.Lock()
			if p.blocked || p.closed {
				p.rejected++
				p.mu.Unlock()
				_ = conn.Close()
				continue
			}
			p.conns[conn] = struct{}{}
			p.wg.Add(1)
			p.mu.Unlock()
			go p.forward(conn)
		}
	}()
	t.Cleanup(func() {
		p.mu.Lock()
		p.closed = true
		for conn := range p.conns {
			_ = conn.Close()
		}
		p.mu.Unlock()
		_ = listener.Close()
		p.wg.Wait()
	})
	return p
}

func (p *remotingTLSProxy) forward(front net.Conn) {
	defer p.wg.Done()
	defer front.Close()
	defer func() {
		p.mu.Lock()
		delete(p.conns, front)
		p.mu.Unlock()
	}()
	back, err := net.DialTimeout("tcp", p.backend, time.Second)
	if err != nil {
		return
	}
	defer back.Close()
	p.mu.Lock()
	if p.blocked || p.closed {
		p.mu.Unlock()
		return
	}
	p.conns[back] = struct{}{}
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		delete(p.conns, back)
		p.mu.Unlock()
	}()
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(back, front)
		_ = back.Close()
		_ = front.Close()
		close(done)
	}()
	_, _ = io.Copy(front, back)
	_ = front.Close()
	_ = back.Close()
	<-done
}

func (p *remotingTLSProxy) partition(blocked bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.blocked = blocked
	if blocked {
		for conn := range p.conns {
			_ = conn.Close()
		}
	}
}

func TestSystem_RemotingTLSClusterRecovery(t *testing.T) {
	ca := testkit.NewTLSAuthority(t)
	backends := []string{clusterTestAddress(t), clusterTestAddress(t)}
	proxies := []*remotingTLSProxy{newRemotingTLSProxy(t, backends[0]), newRemotingTLSProxy(t, backends[1])}
	var systems []*actor.System
	for i := range 2 {
		var seeds []string
		if i == 1 {
			seeds = []string{proxies[0].listener.Addr().String()}
		}
		system := startRemotingTLSSystem(t, ca.Config(t, true),
			vivid.WithActorSystemRemoting(backends[i], proxies[i].listener.Addr().String()),
			vivid.WithActorSystemClusterOptions(
				vivid.WithActorSystemClusterSeeds(seeds...),
				vivid.WithActorSystemClusterLaunchDelay(0),
				vivid.WithActorSystemClusterGossipInterval(50*time.Millisecond),
				vivid.WithActorSystemClusterGracefulShutdownTimeout(500*time.Millisecond),
			),
		)
		_, err := system.ClusterReady(3 * time.Second).Result()
		require.NoError(t, err)
		systems = append(systems, system)
	}
	ref, err := systems[0].ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		if message, ok := ctx.Message().(vivid.ActorRef); ok {
			ctx.Reply(message)
		}
	}))
	require.NoError(t, err)
	before, err := systems[1].Ask(ref, systems[1].Ref(), time.Second).Result()
	require.NoError(t, err)
	require.True(t, systems[1].Ref().Equals(before.(vivid.ActorRef)))
	for _, proxy := range proxies {
		proxy.partition(true)
	}
	require.Eventually(t, func() bool {
		for _, proxy := range proxies {
			proxy.mu.Lock()
			rejected := proxy.rejected
			proxy.mu.Unlock()
			if rejected > 0 {
				return true
			}
		}
		return false
	}, 3*time.Second, 20*time.Millisecond, "确认隔离期间实际拒绝了新连接")
	for _, proxy := range proxies {
		proxy.partition(false)
	}
	message, err := systems[1].CreateRef(systems[1].Ref().GetAddress(), "/after-partition")
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		result, err := systems[1].Ask(ref, message, 500*time.Millisecond).Result()
		return err == nil && message.Equals(result.(vivid.ActorRef))
	}, 5*time.Second, 50*time.Millisecond, "恢复后必须有新的跨节点业务应答，旧成员视图不能证明链路已恢复")
	for _, system := range systems {
		snapshot, err := system.ClusterSnapshot(time.Second).Result()
		require.NoError(t, err)
		require.Equal(t, 2, snapshot.Len())
		for _, member := range snapshot.Members() {
			require.Equal(t, vivid.ClusterMemberStatusUp, member.Status)
		}
	}
}
