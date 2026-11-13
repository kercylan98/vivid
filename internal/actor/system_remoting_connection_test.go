package actor_test

import (
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/actor"
	"github.com/kercylan98/vivid/internal/remoting"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/require"
)

// delayedRemotingProxy 只延迟每个方向的首个数据帧，控制帧仍按 TCP 顺序转发。
type delayedRemotingProxy struct {
	listener net.Listener
	mu       sync.Mutex
	conns    map[net.Conn]struct{}
	closed   bool
	wait     sync.WaitGroup
}

func TestSystem_RemotingConnectionReuse(t *testing.T) {
	for _, first := range []int{0, 1} {
		t.Run(strconv.Itoa(first), func(t *testing.T) {
			addresses := []string{clusterTestAddress(t), clusterTestAddress(t)}
			if addresses[0] > addresses[1] {
				addresses[0], addresses[1] = addresses[1], addresses[0]
			}
			var systems []*actor.System
			for _, address := range addresses {
				system := actor.NewSystem(
					vivid.WithActorSystemRemoting(address),
					vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
				)
				require.NoError(t, system.Start())
				t.Cleanup(func() { require.NoError(t, system.Stop(3*time.Second)) })
				systems = append(systems, system)
			}
			type delivery struct {
				address string
				path    string
			}
			received := make(chan delivery, 128)
			var refs []vivid.ActorRef
			for _, system := range systems {
				ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
					if message, ok := ctx.Message().(vivid.ActorRef); ok {
						if message.GetPath() == "/ask" {
							ctx.Reply(message)
						} else {
							received <- delivery{ctx.Ref().GetAddress(), message.GetPath()}
						}
					}
				}))
				require.NoError(t, err)
				refs = append(refs, ref)
			}
			for _, sender := range []int{first, 1 - first} {
				message, err := systems[sender].CreateRef(addresses[sender], "/ask")
				require.NoError(t, err)
				result, err := systems[sender].Ask(refs[1-sender], message, time.Second).Result()
				require.NoError(t, err, "单向建连后须支持双向请求和回复")
				require.True(t, message.Equals(result.(vivid.ActorRef)))
			}
			for i := range 64 {
				for sender := range 2 {
					message, err := systems[sender].CreateRef(addresses[sender], "/message/"+strconv.Itoa(i))
					require.NoError(t, err)
					systems[sender].Tell(refs[1-sender], message)
				}
			}
			next := make(map[string]int)
			for range 128 {
				message := receiveTellResult(t, received)
				require.Equal(t, "/message/"+strconv.Itoa(next[message.address]), message.path, "稳定连接应保留每个发送方的消息顺序")
				next[message.address]++
			}
			require.Equal(t, 64, next[addresses[0]])
			require.Equal(t, 64, next[addresses[1]])
		})
	}
}

func newDelayedRemotingProxy(t *testing.T, backend string) *delayedRemotingProxy {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	p := &delayedRemotingProxy{listener: listener, conns: make(map[net.Conn]struct{})}
	p.wait.Add(1)
	go func() {
		defer p.wait.Done()
		for {
			front, err := listener.Accept()
			if err != nil {
				return
			}
			p.wait.Add(1)
			go func() {
				defer p.wait.Done()
				defer front.Close()
				back, err := net.DialTimeout("tcp", backend, time.Second)
				if err != nil {
					return
				}
				defer back.Close()
				p.mu.Lock()
				if p.closed {
					p.mu.Unlock()
					return
				}
				p.conns[front], p.conns[back] = struct{}{}, struct{}{}
				p.mu.Unlock()
				defer func() {
					p.mu.Lock()
					delete(p.conns, front)
					delete(p.conns, back)
					p.mu.Unlock()
				}()
				done := make(chan struct{})
				go func() {
					p.forward(back, front)
					_ = front.Close()
					_ = back.Close()
					close(done)
				}()
				p.forward(front, back)
				_ = front.Close()
				_ = back.Close()
				<-done
			}()
		}
	}()
	return p
}

func (*delayedRemotingProxy) forward(destination, source net.Conn) {
	delayed := false
	for {
		frame, err := remoting.ReadFrame(source, nil, remoting.FrameReadLimits{MaxControlLen: 4096, MaxDataLen: 1 << 20})
		if err != nil {
			return
		}
		if frame.Type == remoting.FrameCtrlData && !delayed {
			delayed = true
			time.Sleep(30 * time.Millisecond)
		}
		if _, err := frame.WriteTo(destination); err != nil {
			return
		}
	}
}

func (p *delayedRemotingProxy) Close() {
	_ = p.listener.Close()
	p.mu.Lock()
	p.closed = true
	for conn := range p.conns {
		_ = conn.Close()
	}
	p.mu.Unlock()
	p.wait.Wait()
}

func TestSystem_RemotingSimultaneousFirstMessages(t *testing.T) {
	for range 16 {
		func() {
			var systems []*actor.System
			for range 2 {
				backend := clusterTestAddress(t)
				proxy := newDelayedRemotingProxy(t, backend)
				defer proxy.Close()
				system := actor.NewSystem(
					vivid.WithActorSystemRemoting(backend, proxy.listener.Addr().String()),
					vivid.WithActorSystemRemotingOption(vivid.WithActorSystemRemotingReconnectLimit(0)),
					vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
				)
				require.NoError(t, system.Start())
				defer func() { require.NoError(t, system.Stop(3*time.Second)) }()
				systems = append(systems, system)
			}
			received := make(chan string, 2)
			refs := make([]vivid.ActorRef, 2)
			for i, system := range systems {
				ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
					if message, ok := ctx.Message().(vivid.ActorRef); ok {
						received <- message.GetAddress()
					}
				}))
				require.NoError(t, err)
				refs[i] = ref
			}
			var start, sent sync.WaitGroup
			start.Add(1)
			sent.Add(2)
			for i := range 2 {
				go func() {
					defer sent.Done()
					start.Wait()
					systems[i].Tell(refs[1-i], systems[i].Ref())
				}()
			}
			start.Done()
			sent.Wait()
			seen := make(map[string]bool)
			for range 2 {
				select {
				case address := <-received:
					require.False(t, seen[address], "首次消息不应重复")
					seen[address] = true
				case <-time.After(time.Second):
					t.Fatalf("同时首次发送只收到 %d 条消息", len(seen))
				}
			}
		}()
	}
}
