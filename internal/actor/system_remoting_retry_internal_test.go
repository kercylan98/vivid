package actor

import (
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/remoting"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/require"
)

type retryEventStream struct {
	vivid.EventStream
	observe func(vivid.EventStreamContext, vivid.Message)
}

func (s *retryEventStream) Publish(ctx vivid.EventStreamContext, event vivid.Message) {
	s.EventStream.Publish(ctx, event)
	s.observe(ctx, event)
}

type retryNotification struct {
	ref     vivid.ActorRef
	message vivid.Message
	at      time.Time
}

func receiveRetryResult[T any](t *testing.T, results <-chan T) T {
	t.Helper()
	select {
	case result := <-results:
		return result
	case <-time.After(3 * time.Second):
		t.Fatal("等待重试演练结果超时")
		var result T
		return result
	}
}

func TestSystem_RemotingStaleRetryNotification(t *testing.T) {
	for _, action := range []string{"retry", "takeover", "stop"} {
		t.Run(action, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			defer listener.Close()
			accepted := make(chan time.Time, 16)
			var attempts atomic.Int32
			go func() {
				for {
					conn, err := listener.Accept()
					if err != nil {
						return
					}
					attempts.Add(1)
					accepted <- time.Now()
					_ = conn.SetReadDeadline(time.Now().Add(time.Second))
					_ = new(remoting.Handshake).Wait(conn)
					_ = conn.Close()
				}
			}()
			local, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			address := local.Addr().String()
			require.NoError(t, local.Close())
			const delay = 150 * time.Millisecond
			system := NewSystem(
				vivid.WithActorSystemRemoting(address),
				vivid.WithActorSystemRemotingOption(vivid.WithActorSystemRemotingReconnect(2, delay, 2*delay, 2, false)),
				vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
			)
			defer func() {
				err := system.Stop(time.Second)
				require.True(t, err == nil || errors.Is(err, vivid.ErrorActorSystemAlreadyStopped), "%v", err)
			}()
			ready := make(chan retryNotification, 1)
			failed := 0
			var first vivid.Message
			system.eventStream = &retryEventStream{EventStream: system.eventStream, observe: func(eventContext vivid.EventStreamContext, event vivid.Message) {
				failure, ok := event.(ves.RemotingConnectionFailedEvent)
				if !ok || failure.Address != listener.Addr().String() {
					return
				}
				ctx := eventContext.(*Context)
				failed++
				if failed == 1 {
					// 在端点消息线程记录真实调度通知，保留其消息体供下一轮退避时重放。
					behavior := ctx.actor.OnReceive
					ctx.Become(func(current vivid.ActorContext) {
						if first == nil && fmt.Sprintf("%T", current.Message()) == "remoting.endpointRetryConnect" {
							first = current.Message()
						}
						behavior(current)
					})
				}
				if failed == 2 {
					ready <- retryNotification{ref: ctx.Ref(), message: first, at: time.Now()}
				}
			}}
			require.NoError(t, system.Start())
			target, err := system.CreateRef(listener.Addr().String(), "/retry")
			require.NoError(t, err)
			system.Tell(target, system.Ref())
			receiveRetryResult(t, accepted)
			receiveRetryResult(t, accepted)
			notification := receiveRetryResult(t, ready)
			require.NotNil(t, notification.message, "必须重放实际定时器投递的通知")
			if action == "takeover" {
				conn, err := net.DialTimeout("tcp", address, time.Second)
				require.NoError(t, err)
				defer conn.Close()
				require.NoError(t, conn.SetDeadline(time.Now().Add(time.Second)))
				require.NoError(t, (&remoting.Handshake{AdvertiseAddr: listener.Addr().String()}).Send(conn))
				require.NoError(t, new(remoting.Handshake).Wait(conn))
				frame, err := remoting.ReadFrame(conn, nil, remoting.FrameReadLimits{MaxDataLen: 4096})
				require.NoError(t, err)
				require.Equal(t, remoting.FrameCtrlData, frame.Type, "已有入站连接应接管待发送消息")
			}
			// 多次重放上一轮真实通知，不能提前执行新退避或覆盖取消结果。
			for range 8 {
				system.Tell(notification.ref, notification.message)
			}
			if action == "retry" {
				next := receiveRetryResult(t, accepted)
				require.GreaterOrEqual(t, next.Sub(notification.at), 2*delay-10*time.Millisecond)
				require.Equal(t, int32(3), attempts.Load(), "首次拨号与两次额外重试分别计数")
				t.Logf("retry_delay=%v observed_delay=%v attempts=%d", 2*delay, next.Sub(notification.at), attempts.Load())
			} else if action == "stop" {
				require.NoError(t, system.Stop(time.Second))
			}
			select {
			case <-accepted:
				t.Fatal("重试耗尽、连接接管或停止后仍然拨号")
			case <-time.After(4 * delay):
			}
			if action != "retry" {
				require.Equal(t, int32(2), attempts.Load())
			}
		})
	}
}
