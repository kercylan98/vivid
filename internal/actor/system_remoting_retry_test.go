package actor_test

import (
	"errors"
	"net"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/actor"
	"github.com/kercylan98/vivid/internal/remoting"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/require"
)

func TestSystem_RemotingRetryBudget(t *testing.T) {
	for _, test := range []struct {
		limit int
		stop  bool
	}{{0, false}, {2, false}, {2, true}} {
		t.Run(strconv.Itoa(test.limit)+"/stop="+strconv.FormatBool(test.stop), func(t *testing.T) {
			limit := test.limit
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			defer listener.Close()
			var attempts atomic.Int32
			accepted := make(chan time.Time, 32)
			go func() {
				for {
					conn, err := listener.Accept()
					if err != nil {
						return
					}
					attempts.Add(1)
					accepted <- time.Now()
					_ = conn.SetReadDeadline(time.Now().Add(time.Second))
					_, _ = remoting.ReadFrame(conn, nil, remoting.FrameReadLimits{MaxDataLen: 1024})
					_ = conn.Close() // 拒绝握手，使每次尝试确定失败。
				}
			}()
			const delay = 80 * time.Millisecond
			system := actor.NewSystem(
				vivid.WithActorSystemRemoting(clusterTestAddress(t)),
				vivid.WithActorSystemRemotingOption(vivid.WithActorSystemRemotingReconnect(limit, delay, delay, 1, false)),
				vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
			)
			require.NoError(t, system.Start())
			defer func() {
				err := system.Stop(time.Second)
				require.True(t, err == nil || errors.Is(err, vivid.ErrorActorSystemAlreadyStopped), "%v", err)
			}()
			target, err := system.CreateRef(listener.Addr().String(), "/retry")
			require.NoError(t, err)
			ready := make(chan struct{})
			failed := make(chan struct{}, 32)
			dead := make(chan struct{}, 32)
			_, err = system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
				switch message := ctx.Message().(type) {
				case *vivid.OnLaunch:
					ctx.EventStream().Subscribe(ctx, ves.RemotingConnectionFailedEvent{})
					ctx.EventStream().Subscribe(ctx, ves.DeathLetterEvent{})
					close(ready)
				case ves.RemotingConnectionFailedEvent:
					if message.Address == target.GetAddress() {
						failed <- struct{}{}
					}
				case ves.DeathLetterEvent:
					if target.Equals(message.Envelope.Receiver()) {
						dead <- struct{}{}
					}
				}
			}))
			require.NoError(t, err)
			receiveTellResult(t, ready)
			system.Tell(target, system.Ref())
			first := receiveTellResult(t, accepted)
			receiveTellResult(t, failed)
			if test.stop {
				require.NoError(t, system.Stop(time.Second))
				select {
				case <-accepted:
					t.Fatal("停止后仍执行定时重试")
				case <-time.After(2 * delay):
				}
				require.Equal(t, int32(1), attempts.Load())
				return
			}
			pending := 1
			if limit > 0 {
				// 退避期间的新消息只入队，不能绕过已安排的重试。
				for range 8 {
					system.Tell(target, system.Ref())
					pending++
				}
				next := receiveTellResult(t, accepted)
				require.GreaterOrEqual(t, next.Sub(first), delay-10*time.Millisecond)
				for range limit - 1 {
					receiveTellResult(t, accepted)
				}
			}
			for range pending {
				receiveTellResult(t, dead)
			}
			require.Equal(t, int32(1+limit), attempts.Load(), "首次拨号与额外重试分别计数")
			select {
			case <-accepted:
				t.Fatal("重试耗尽后仍然拨号")
			case <-time.After(2 * delay):
			}
		})
	}
}
