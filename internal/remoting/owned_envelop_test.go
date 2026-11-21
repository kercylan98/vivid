package remoting

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/serialization"
	"github.com/stretchr/testify/require"
)

func TestOwnedEnvelop_CompletionAfterWriters(t *testing.T) {
	for _, written := range []bool{false, true} {
		t.Run(map[bool]string{false: "abandoned", true: "written"}[written], func(t *testing.T) {
			admission := NewOutboundAdmission(vivid.NewActorSystemRemotingOptions(), nil, nil)
			completed := make(chan bool, 2)
			original := newAdmissionTestEnvelop("a:1")
			owned := NewOwnedEnvelop(original, func(result bool) {
				// 完成通知允许重新访问准入，不能在任一状态锁内调用。
				admission.Stop()
				completed <- result
			})
			pending, err := admission.reserve(owned)
			require.NoError(t, err)
			require.True(t, beginOutboundWrite(pending))
			require.True(t, beginOutboundWrite(pending), "旧连接结束前，新连接可能已开始重发")
			if written {
				_, ok := completeOutboundEnvelop(pending)
				require.True(t, ok)
			} else {
				admission.Stop()
			}
			require.Empty(t, admission.pending)
			require.False(t, beginOutboundWrite(pending))
			endOutboundWrite(pending)
			require.Empty(t, completed)
			endOutboundWrite(pending)
			require.Equal(t, written, <-completed)
			_, ok := completeOutboundEnvelop(pending)
			require.False(t, ok)
			failOutboundEnvelop(nil, owned)
			require.Empty(t, completed)
			require.Nil(t, owned.(*ownedEnvelop).complete, "终结信封不保留回调捕获的节点资源")
		})
	}
}

func TestOwnedEnvelop_FailureHandlerRetainsPayload(t *testing.T) {
	var released bool
	original := newAdmissionTestEnvelop("a:1")
	owned := NewOwnedEnvelop(original, func(written bool) {
		require.False(t, written)
		released = true
	})
	handler := &admissionTestHandler{failed: func(envelop vivid.Envelop) {
		require.Same(t, original, envelop)
		require.False(t, released, "失败通知也可能读取负载")
		failOutboundEnvelop(nil, owned)
	}}
	failOutboundEnvelop(handler, owned)
	require.True(t, released)
}

func TestOwnedEnvelop_MailboxRejection(t *testing.T) {
	for _, mode := range []string{"nil", "canceled", "no_route", "address", "capacity", "stopped"} {
		t.Run(mode, func(t *testing.T) {
			completed := make(chan bool, 2)
			address := "a:1"
			if mode == "address" {
				address = ""
			}
			owned := NewOwnedEnvelop(newAdmissionTestEnvelop(address), func(written bool) { completed <- written })
			admission := NewOutboundAdmission(vivid.ActorSystemRemotingOptions{MaxPendingEnvelops: 1}, nil, nil)
			m := NewMailbox(context.Background(), &admissionTestLiaison{tell: func(vivid.ActorRef, vivid.Message) {
				t.Fatal("已拒绝的消息不能进入 Remoting")
			}}, &admissionTestRef{address: "local:1"}, nil, admission)
			switch mode {
			case "nil":
				m = nil
			case "canceled":
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				m.ctx = ctx
			case "no_route":
				m.remotingRef = nil
			case "capacity":
				_, err := admission.reserve(newAdmissionTestEnvelop(address))
				require.NoError(t, err)
			case "stopped":
				admission.Stop()
			}
			m.Enqueue(owned)
			require.False(t, <-completed)
			failOutboundEnvelop(nil, owned)
			require.Empty(t, completed)
			admission.Stop()
		})
	}
}

func TestOwnedEnvelop_QueuedDeathLetter(t *testing.T) {
	admission := NewOutboundAdmission(vivid.NewActorSystemRemotingOptions(), nil, nil)
	completed := make(chan bool, 2)
	owned := NewOwnedEnvelop(newAdmissionTestEnvelop("a:1"), func(written bool) { completed <- written })
	pending, err := admission.reserve(owned)
	require.NoError(t, err)
	require.True(t, FailOutboundMessage(pending))
	require.False(t, <-completed)
	require.True(t, FailOutboundMessage(pending))
	require.Empty(t, completed)
	require.Empty(t, admission.pending)
	require.False(t, beginOutboundWrite(pending))
}

func TestOwnedEnvelop_ConcurrentStopAndWrite(t *testing.T) {
	for range 100 {
		admission := NewOutboundAdmission(vivid.NewActorSystemRemotingOptions(), nil, nil)
		var active, calls, premature atomic.Int32
		owned := NewOwnedEnvelop(newAdmissionTestEnvelop("a:1"), func(bool) {
			calls.Add(1)
			if active.Load() != 0 {
				premature.Add(1)
			}
		})
		pending, err := admission.reserve(owned)
		require.NoError(t, err)
		var workers sync.WaitGroup
		for range 20 {
			workers.Go(func() {
				if beginOutboundWrite(pending) {
					active.Add(1)
					active.Add(-1)
					endOutboundWrite(pending)
				}
			})
		}
		workers.Go(admission.Stop)
		workers.Go(func() { completeOutboundEnvelop(pending) })
		workers.Wait()
		require.EqualValues(t, 1, calls.Load())
		require.Zero(t, premature.Load())
	}
}

type ownedWriterContext struct {
	vivid.ActorContext
	notices chan vivid.Message
}

func (c *ownedWriterContext) Tell(_ vivid.ActorRef, message vivid.Message) { c.notices <- message }
func (c *ownedWriterContext) Ref() vivid.ActorRef                          { return nil }

type ownedWriteConn struct {
	net.Conn
	entered chan struct{}
}

func (c *ownedWriteConn) Write(data []byte) (int, error) {
	close(c.entered)
	return c.Conn.Write(data)
}

func TestOwnedEnvelop_StopDuringWriter(t *testing.T) {
	for _, mode := range []string{"encoding", "writing"} {
		t.Run(mode, func(t *testing.T) {
			entered, resume, ended := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var release sync.Once
			defer release.Do(func() { close(resume) })
			failure := errors.New("write interrupted")
			codec := serialization.NewVividCodec(nil)
			require.NoError(t, codec.RegisterMessageWithEncoderAndDecoder("TEST", "string", "", serialization.MessageEncoderFN(func(w *serialization.Writer, _ any) error {
				if mode == "encoding" {
					close(entered)
					<-resume
					return failure
				}
				w.WriteString("payload")
				return w.Err()
			}), nil))
			completed := make(chan bool, 2)
			admission := NewOutboundAdmission(vivid.NewActorSystemRemotingOptions(), nil, nil)
			pending, err := admission.reserve(NewOwnedEnvelop(newAdmissionTestEnvelop("a:1"), func(written bool) { completed <- written }))
			require.NoError(t, err)
			conn, peer := net.Pipe()
			defer conn.Close()
			defer peer.Close()
			session := newSession("a:1", "local:1", &ownedWriteConn{Conn: conn, entered: entered}, sessionRoleOutbound)
			writer := newEndpointWriter(1, session, codec, nil)
			ctx := &ownedWriterContext{notices: make(chan vivid.Message, 2)}
			go func() { defer close(ended); writer.onSend(ctx, endpointWriterSend{envelop: pending}) }()
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("writer 未进入预定阻塞点")
			}
			admission.Stop()
			require.Empty(t, completed, "停止准入不代表 writer 已返回")
			release.Do(func() { close(resume) })
			require.NoError(t, session.Close())
			select {
			case <-ended:
			case <-time.After(time.Second):
				t.Fatal("writer 未退出")
			}
			require.False(t, <-completed)
			require.IsType(t, endpointWriterFailed{}, <-ctx.notices)
			require.Empty(t, completed)
		})
	}
}

func TestOwnedEnvelop_WireFormatUnchanged(t *testing.T) {
	original := newAdmissionTestEnvelop("a:1")
	codec := serialization.NewVividCodec(budgetExternalCodec{})
	want, err := encodeEnvelop(codec, original)
	require.NoError(t, err)
	actual, err := encodeEnvelop(codec, NewOwnedEnvelop(original, nil))
	require.NoError(t, err)
	require.Equal(t, want, actual)
}

// BenchmarkOwnedEnvelop_WriteAndComplete 仅测所有权和准入流转，不含编码、网络或资源自身的释放成本。
func BenchmarkOwnedEnvelop_WriteAndComplete(b *testing.B) {
	admission := NewOutboundAdmission(vivid.NewActorSystemRemotingOptions(), nil, nil)
	original := newAdmissionTestEnvelop("a:1")
	b.ReportAllocs()
	for b.Loop() {
		owned := NewOwnedEnvelop(original, func(bool) {})
		pending, err := admission.reserve(owned)
		if err != nil {
			b.Fatal(err)
		}
		if !beginOutboundWrite(pending) {
			b.Fatal("发送所有权已结束")
		}
		endOutboundWrite(pending)
		completeOutboundEnvelop(pending)
	}
}
