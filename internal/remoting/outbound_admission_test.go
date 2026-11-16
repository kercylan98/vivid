package remoting

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/serialization"
	"github.com/stretchr/testify/require"
)

type admissionTestRef struct {
	vivid.ActorRef
	address string
}

func (r *admissionTestRef) GetAddress() string { return r.address }
func (r *admissionTestRef) String() string     { return r.address + "@/target" }

type admissionTestEnvelop struct {
	vivid.Envelop
	receiver vivid.ActorRef
	message  vivid.Message
	system   bool
}

func (e *admissionTestEnvelop) System() bool             { return e.system }
func (e *admissionTestEnvelop) Sender() vivid.ActorRef   { return nil }
func (e *admissionTestEnvelop) Receiver() vivid.ActorRef { return e.receiver }
func (e *admissionTestEnvelop) Message() vivid.Message   { return e.message }

func newAdmissionTestEnvelop(address string) vivid.Envelop {
	return &admissionTestEnvelop{receiver: &admissionTestRef{address: address}, message: "hello"}
}

type admissionTestLiaison struct {
	vivid.ActorLiaison
	tell func(vivid.ActorRef, vivid.Message)
}

func (l *admissionTestLiaison) Tell(ref vivid.ActorRef, message vivid.Message) { l.tell(ref, message) }

type admissionTestHandler struct {
	failed func(vivid.Envelop)
}

func (h *admissionTestHandler) HandleFailedRemotingEnvelop(envelop vivid.Envelop) { h.failed(envelop) }

func TestOutboundAdmission_DefaultsAndOptions(t *testing.T) {
	for _, options := range []vivid.ActorSystemRemotingOptions{{}, {MaxPendingEnvelops: -1, MaxTotalPendingEnvelops: -1}, vivid.NewActorSystemRemotingOptions(
		vivid.WithActorSystemRemotingMaxTotalPendingEnvelops(0),
	)} {
		admission := NewOutboundAdmission(options, nil, nil)
		require.Equal(t, 1024, admission.perAddress)
		require.Equal(t, 16384, admission.total)
	}
	options := vivid.NewActorSystemRemotingOptions(vivid.WithActorSystemRemotingMaxPendingEnvelops(2), vivid.WithActorSystemRemotingMaxTotalPendingEnvelops(3))
	admission := NewOutboundAdmission(options, nil, nil)
	first, err := admission.reserve(newAdmissionTestEnvelop("a:1"))
	require.NoError(t, err)
	_, err = admission.reserve(newAdmissionTestEnvelop("a:1"))
	require.NoError(t, err)
	_, err = admission.reserve(newAdmissionTestEnvelop("a:1"))
	require.ErrorIs(t, err, vivid.ErrorRemotingPendingEnvelopsFull)
	_, err = admission.reserve(newAdmissionTestEnvelop("b:1"))
	require.NoError(t, err)
	_, err = admission.reserve(newAdmissionTestEnvelop("c:1"))
	require.ErrorIs(t, err, vivid.ErrorRemotingTotalPendingEnvelopsFull)
	completeOutboundEnvelop(first)
	_, err = admission.reserve(newAdmissionTestEnvelop("c:1"))
	require.NoError(t, err, "完成一个地址的写入后，总名额可供其他地址使用")
	completeOutboundEnvelop(first)
	require.Len(t, admission.pending, 3, "旧完成结果不能释放新名额")
	admission.Stop()
	require.Empty(t, admission.pending)
	require.Empty(t, admission.addresses)
}

func TestMailbox_AdmissionBeforeDeliveryAndStop(t *testing.T) {
	entered, resume := make(chan struct{}), make(chan struct{})
	delivered := make(chan *outboundEnvelop, 1)
	original := newAdmissionTestEnvelop("a:1")
	failed := make(chan vivid.Envelop, 3)
	var admission *OutboundAdmission
	handler := &admissionTestHandler{failed: func(envelop vivid.Envelop) {
		// 失败回调可以同步重新访问准入，不能持锁调用。
		admission.mu.Lock()
		admission.mu.Unlock()
		failed <- envelop
	}}
	admission = NewOutboundAdmission(vivid.ActorSystemRemotingOptions{MaxPendingEnvelops: 1, MaxTotalPendingEnvelops: 1}, handler, nil)
	liaison := &admissionTestLiaison{tell: func(_ vivid.ActorRef, message vivid.Message) {
		close(entered)
		<-resume
		delivered <- message.(*outboundEnvelop)
	}}
	mailbox := NewMailbox(context.Background(), liaison, &admissionTestRef{}, handler, admission)
	var group sync.WaitGroup
	group.Go(func() { mailbox.Enqueue(original) })
	<-entered
	// 第一条尚未进入 Remoting 邮箱，第二个邮箱仍须共享同一个总额度。
	other := NewMailbox(context.Background(), liaison, &admissionTestRef{}, handler, admission)
	rejected := newAdmissionTestEnvelop("b:1")
	other.Enqueue(rejected)
	require.Same(t, rejected, <-failed)
	admission.Stop()
	require.Same(t, original, <-failed)
	close(resume)
	group.Wait()
	late := <-delivered
	require.False(t, outboundEnvelopActive(late))
	require.True(t, FailOutboundMessage(late))
	_, completed := completeOutboundEnvelop(late)
	require.False(t, completed)
	admission.Stop()
	require.Empty(t, failed, "停止与迟到结果只能产生一次原始死信")
	require.Empty(t, admission.pending)
	require.Empty(t, admission.addresses)
}

func TestOutboundAdmission_ConcurrentReserveAndStop(t *testing.T) {
	var failed atomic.Int32
	admission := NewOutboundAdmission(vivid.ActorSystemRemotingOptions{MaxPendingEnvelops: 4, MaxTotalPendingEnvelops: 12}, &admissionTestHandler{failed: func(vivid.Envelop) { failed.Add(1) }}, nil)
	var group sync.WaitGroup
	var accepted atomic.Int32
	var rejected atomic.Int32
	var succeeded atomic.Int32
	var attempted sync.WaitGroup
	attempted.Add(128)
	start, release := make(chan struct{}), make(chan struct{})
	for index := range 128 {
		group.Go(func() {
			<-start
			pending, err := admission.reserve(newAdmissionTestEnvelop(fmt.Sprintf("node%d:1", index%8)))
			if err != nil {
				rejected.Add(1)
				attempted.Done()
				return
			}
			accepted.Add(1)
			attempted.Done()
			<-release
			if index%2 == 0 {
				if _, completed := completeOutboundEnvelop(pending); completed {
					succeeded.Add(1)
				}
			} else {
				failOutboundEnvelop(admission.handler, pending)
			}
		})
	}
	close(start)
	// 不释放任何申请，必须同时约束并发生产者的总保留数量。
	attempted.Wait()
	require.LessOrEqual(t, len(admission.pending), 12)
	for _, count := range admission.addresses {
		require.LessOrEqual(t, count, 4)
	}
	require.Equal(t, int32(12), accepted.Load())
	group.Go(admission.Stop)
	close(release)
	group.Wait()
	require.Equal(t, accepted.Load(), failed.Load()+succeeded.Load())
	require.Empty(t, admission.pending)
	require.Empty(t, admission.addresses)
	_, err := admission.reserve(newAdmissionTestEnvelop("a:1"))
	require.ErrorIs(t, err, vivid.ErrorRemotingStopped)
}

func TestOutboundAdmission_WireAndFailurePaths(t *testing.T) {
	for _, action := range []string{"encode", "exhausted", "schedule_failed", "kill", "dropped", "buffer_full", "routing_failed", "ack"} {
		t.Run(action, func(t *testing.T) {
			e, ctx := newRetryTestEndpoint(t)
			handler := &stopTestHandler{}
			admission := NewOutboundAdmission(vivid.NewActorSystemRemotingOptions(), handler, nil)
			original := newAdmissionTestEnvelop(e.address)
			pending, err := admission.reserve(original)
			require.NoError(t, err)
			e.envelopHandler = handler
			e.outboundBuffer.items = []vivid.Envelop{pending}
			e.association = &endpointAssociation{id: 1, writer: ctx.ref}
			switch action {
			case "encode":
				codec := serialization.NewVividCodec(budgetExternalCodec{})
				encoded, err := encodeEnvelop(codec, pending)
				require.NoError(t, err)
				expected, err := encodeEnvelop(codec, original)
				require.NoError(t, err)
				require.Equal(t, expected, encoded, "准入包装不得改变网络字段")
				e.onWriterFailed(ctx, endpointWriterFailed{associationID: 1, writer: ctx.ref, err: errors.New("encode failed"), dropCurrent: true})
			case "exhausted":
				e.retryPolicy.limit = 0
				e.scheduleRetry(ctx, errors.New("connect failed"))
			case "schedule_failed":
				ctx.scheduler.onceErr = errors.New("schedule failed")
				e.scheduleRetry(ctx, errors.New("connect failed"))
			case "kill":
				e.onKill(ctx)
			case "dropped":
				require.True(t, FailOutboundMessage(pending))
				require.True(t, FailOutboundMessage(pending))
			case "buffer_full":
				e.outboundBuffer.limit = 1
				e.outboundBuffer.items = []vivid.Envelop{original}
				e.onEnvelop(ctx, pending)
			case "routing_failed":
				e.endpointManager.Stop()
				r := &Remoting{endpointManager: e.endpointManager, envelopHandler: handler}
				r.onEnvelop(ctx, pending)
			case "ack":
				e.onWriterAck(ctx, endpointWriterAck{associationID: 1, writer: ctx.ref})
			}
			require.Empty(t, admission.pending)
			require.Empty(t, admission.addresses)
			if action != "ack" {
				require.Len(t, handler.failed, 1)
				require.Same(t, original, handler.failed[0])
			} else {
				require.Empty(t, handler.failed)
			}
			admission.Stop()
		})
	}
}

func TestOutboundAdmission_RetryKeepsPermit(t *testing.T) {
	e, ctx := newRetryTestEndpoint(t)
	admission := NewOutboundAdmission(vivid.ActorSystemRemotingOptions{MaxPendingEnvelops: 1}, nil, nil)
	pending, err := admission.reserve(newAdmissionTestEnvelop(e.address))
	require.NoError(t, err)
	e.outboundBuffer.items = []vivid.Envelop{pending}
	e.scheduleRetry(ctx, errors.New("transient failure"))
	require.True(t, e.retryScheduled)
	_, err = admission.reserve(newAdmissionTestEnvelop(e.address))
	require.ErrorIs(t, err, vivid.ErrorRemotingPendingEnvelopsFull, "进入退避不能提前释放待发信封")
	admission.Stop()
	// 停止后投递至端点或 Writer 的旧消息不得再编码或拨号。
	e.onEnvelop(nil, pending)
	(&endpointWriter{}).onSend(nil, endpointWriterSend{envelop: pending})
	(&Remoting{}).onEnvelop(nil, pending)
}

func BenchmarkOutboundAdmission_ReserveAndComplete(b *testing.B) {
	for _, parallel := range []bool{false, true} {
		b.Run(fmt.Sprint(parallel), func(b *testing.B) {
			admission := NewOutboundAdmission(vivid.NewActorSystemRemotingOptions(), nil, nil)
			envelop := newAdmissionTestEnvelop("a:1")
			b.ReportAllocs()
			if parallel {
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						pending, err := admission.reserve(envelop)
						if err != nil {
							b.Fatal(err)
						}
						completeOutboundEnvelop(pending)
					}
				})
			} else {
				for b.Loop() {
					pending, _ := admission.reserve(envelop)
					completeOutboundEnvelop(pending)
				}
			}
		})
	}
}
