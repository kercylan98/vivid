package gossip_test

import (
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/gossip"
	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/internal/remoting"
	"github.com/kercylan98/vivid/pkg/bootstrap"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/require"
)

// gossipNetwork 以真实 TCP 代理隔离两个模拟机房，只截断跨组连接，不修改 Gossip 消息内容。
type gossipNetwork struct {
	mu       sync.Mutex
	groups   map[string]int
	links    map[*gossipLink]struct{}
	blocked  bool
	closed   bool
	rejected int
	wg       sync.WaitGroup
}

type gossipLink struct {
	front net.Conn
	back  net.Conn
	cross bool
}

func (n *gossipNetwork) partition(blocked bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.blocked = blocked
	if blocked {
		for link := range n.links {
			if link.cross {
				_ = link.front.Close()
				_ = link.back.Close()
			}
		}
	}
}

func (n *gossipNetwork) proxy(listener net.Listener, backend string, group int) {
	defer n.wg.Done()
	for {
		front, err := listener.Accept()
		if err != nil {
			return
		}
		n.wg.Add(1)
		go n.forward(front, backend, group)
	}
}

func (n *gossipNetwork) forward(front net.Conn, backend string, group int) {
	defer n.wg.Done()
	defer front.Close()
	_ = front.SetReadDeadline(time.Now().Add(time.Second))
	frame, err := remoting.ReadFrame(front, nil, remoting.FrameReadLimits{MaxControlLen: 4096, MaxDataLen: 1024})
	if err != nil || frame.Type != remoting.FrameCtrlHandshake {
		return
	}
	_ = front.SetReadDeadline(time.Time{})
	n.mu.Lock()
	source, known := n.groups[string(frame.Data)]
	cross := known && source != group
	if n.closed || !known || n.blocked && cross {
		if cross && n.blocked {
			n.rejected++
		}
		n.mu.Unlock()
		return
	}
	n.mu.Unlock()
	back, err := net.DialTimeout("tcp", backend, time.Second)
	if err != nil {
		return
	}
	defer back.Close()
	link := &gossipLink{front: front, back: back, cross: cross}
	n.mu.Lock()
	if n.closed || n.blocked && cross {
		n.mu.Unlock()
		return
	}
	n.links[link] = struct{}{}
	n.mu.Unlock()
	defer func() {
		n.mu.Lock()
		delete(n.links, link)
		n.mu.Unlock()
	}()
	if _, err = frame.WriteTo(back); err != nil {
		return
	}
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

func TestActor_GossipPartitionRecovery(t *testing.T) {
	for _, initiallyBlocked := range []bool{true, false} {
		name := "connected"
		if initiallyBlocked {
			name = "isolated_bootstrap"
		}
		t.Run(name, func(t *testing.T) {
			network := &gossipNetwork{groups: make(map[string]int), links: make(map[*gossipLink]struct{}), blocked: initiallyBlocked}
			var proxies []net.Listener
			var systems []vivid.PrimaryActorSystem
			var refs []vivid.ActorRef
			var addresses, backends []string
			t.Cleanup(func() {
				network.partition(false)
				var stopped sync.WaitGroup
				stopErrors := make(chan error, len(systems))
				for _, system := range systems {
					stopped.Add(1)
					go func() { defer stopped.Done(); stopErrors <- system.Stop(3 * time.Second) }()
				}
				stopped.Wait()
				close(stopErrors)
				for err := range stopErrors {
					if err != nil {
						t.Errorf("stop actor system: %v", err)
					}
				}
				network.mu.Lock()
				network.closed = true
				for link := range network.links {
					_ = link.front.Close()
					_ = link.back.Close()
				}
				network.mu.Unlock()
				for _, listener := range proxies {
					_ = listener.Close()
				}
				network.wg.Wait()
			})
			for i := 0; i < 4; i++ {
				proxy, err := net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)
				proxies = append(proxies, proxy)
				addresses = append(addresses, proxy.Addr().String())
				// 为后端监听申请临时端口，测试结束时由系统释放。
				backend, err := net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)
				backends = append(backends, backend.Addr().String())
				require.NoError(t, backend.Close())
				network.groups[addresses[i]] = i / 2
			}
			for i := 0; i < 4; i++ {
				network.wg.Add(1)
				go network.proxy(proxies[i], backends[i], i/2)
				system := bootstrap.NewActorSystem(
					vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
					vivid.WithActorSystemRemoting(backends[i], addresses[i]),
					vivid.WithActorSystemRemotingOption(
						vivid.WithActorSystemRemotingReconnect(2, 20*time.Millisecond, 100*time.Millisecond, 2, false),
						vivid.WithActorSystemRemotingReadTimeout(500*time.Millisecond),
						vivid.WithActorSystemRemotingHeartbeatInterval(100*time.Millisecond),
					),
				)
				require.NoError(t, system.Start())
				systems = append(systems, system)
			}
			for i, system := range systems {
				seeds := addresses
				if i/2 == 1 {
					seeds = addresses[2:] // 仅一侧保留跨机房种子，也应能在恢复后相互发现。
				}
				opts := vivid.NewActorSystemClusterOptions(
					vivid.WithActorSystemClusterName("vivid-test"),
					vivid.WithActorSystemClusterBootstrap(i%2 == 0),
					vivid.WithActorSystemClusterSeeds(seeds...),
					vivid.WithActorSystemClusterJoinTimeout(500*time.Millisecond),
					vivid.WithActorSystemClusterLaunchDelay(0),
					vivid.WithActorSystemClusterGossipInterval(50*time.Millisecond),
					vivid.WithActorSystemClusterGossipPeersCount(1),
					vivid.WithActorSystemClusterProbeTimeout(150*time.Millisecond),
					vivid.WithActorSystemClusterReachabilityTTL(3*time.Second),
					vivid.WithActorSystemClusterGracefulShutdownTimeout(time.Second),
				)
				member := gossip.NewSnapshotActor(opts)
				ref, err := system.ActorOf(member)
				require.NoError(t, err, "node: %d", i)
				refs = append(refs, ref)
				// 先确认各组初始节点就绪，再让后续节点加入；不依赖失败后隐式自举。
				_, err = member.Readiness().Wait(system, 5*time.Second).Result()
				if err != nil {
					for j, ref := range refs {
						result, queryErr := systems[j].Ask(ref, gossip.TestSnapshotRequest{}, time.Second).Result()
						if queryErr == nil {
							snapshot := result.(gossip.SnapshotResponse)
							t.Logf("join node %d address=%s seeds=%v; node %d status=%s members=%s pings=%v", i, addresses[i], seeds, j, snapshot.Status, snapshot.View.Members().Fingerprint(), snapshot.PingCount)
						}
					}
				}
				require.NoError(t, err, "join node: %d", i)
			}
			read := func(i int) (gossip.SnapshotResponse, bool) {
				result, err := systems[i].Ask(refs[i], gossip.TestSnapshotRequest{}, time.Second).Result()
				if err != nil {
					return gossip.SnapshotResponse{}, false
				}
				snapshot, ok := result.(gossip.SnapshotResponse)
				return snapshot, ok
			}
			waitMembers := func(count int) {
				t.Helper()
				defer func() {
					if t.Failed() {
						for i := range systems {
							if snapshot, ok := read(i); ok {
								t.Logf("node %d status=%s members=%s pings=%v", i, snapshot.Status, snapshot.View.Members().Fingerprint(), snapshot.PingCount)
							}
						}
						network.mu.Lock()
						t.Logf("network rejected=%d links=%d", network.rejected, len(network.links))
						network.mu.Unlock()
					}
				}()
				require.Eventually(t, func() bool {
					fingerprint := ""
					for i := range systems {
						snapshot, ok := read(i)
						if !ok || snapshot.Status != endpoint.StatusUp || len(snapshot.View.Members().List()) != count {
							return false
						}
						for _, member := range snapshot.View.Members().List() {
							if member.Status != endpoint.StatusUp {
								return false
							}
						}
						if count == 4 {
							current := snapshot.View.Fingerprint()
							if fingerprint != "" && fingerprint != current {
								return false
							}
							fingerprint = current
						}
					}
					return true
				}, 8*time.Second, 50*time.Millisecond)
				t.Logf("all four nodes have %d UP members", count)
			}
			waitReachability := func(state vivid.ClusterReachability) {
				t.Helper()
				require.Eventually(t, func() bool {
					for i := range systems {
						snapshot, ok := read(i)
						if !ok {
							return false
						}
						count := 0
						for _, member := range snapshot.Snapshot.Members() {
							if network.groups[member.Address] != i/2 {
								count++
								if member.Reachability != state || member.Status != vivid.ClusterMemberStatusUp {
									return false
								}
							}
						}
						if count != 2 {
							return false
						}
					}
					return true
				}, 8*time.Second, 50*time.Millisecond)
			}
			if initiallyBlocked {
				waitMembers(2)
				network.mu.Lock()
				rejected := network.rejected
				network.mu.Unlock()
				require.Positive(t, rejected, "必须确认跨机房连接确实被拦截")
			} else {
				waitMembers(4)
				waitReachability(vivid.ClusterReachabilityReachable)
				network.partition(true)
				waitReachability(vivid.ClusterReachabilityUnreachable)
			}
			// 隔离稳定后记录跨机房 Ping 数；成员 UP 不代表链路可达。
			time.Sleep(300 * time.Millisecond)
			counts := make([]map[string]uint64, 4)
			for i := range systems {
				snapshot, ok := read(i)
				require.True(t, ok)
				counts[i] = snapshot.PingCount
			}
			time.Sleep(300 * time.Millisecond)
			for i := range systems {
				snapshot, ok := read(i)
				require.True(t, ok)
				for j, address := range addresses {
					if i/2 != j/2 {
						require.Equal(t, counts[i][address], snapshot.PingCount[address], "隔离期间跨机房 Ping 不应到达")
					}
				}
			}
			healedAt := time.Now()
			network.partition(false)
			waitMembers(4)
			waitReachability(vivid.ClusterReachabilityReachable)
			// 视图可能保留了旧成员，必须另外确认恢复后收到新的跨机房 Ping。
			require.Eventually(t, func() bool {
				for i := range systems {
					snapshot, ok := read(i)
					if !ok {
						return false
					}
					var before, after uint64
					for j, address := range addresses {
						if i/2 != j/2 {
							before += counts[i][address]
							after += snapshot.PingCount[address]
						}
					}
					if after <= before {
						return false
					}
				}
				return true
			}, 8*time.Second, 50*time.Millisecond)
			t.Logf("cross-datacenter gossip recovered in %s", time.Since(healedAt))
		})
	}
}
