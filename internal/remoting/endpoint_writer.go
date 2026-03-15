package remoting

import (
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/serialization"
	"github.com/kercylan98/vivid/pkg/metrics"
)

var (
	_ vivid.Actor = (*endpointWriter)(nil)
)

type endpointWriterSend struct {
	envelop vivid.Envelop
}

type endpointWriterAck struct {
	associationID uint64
	writer        vivid.ActorRef
}

type endpointWriterFailed struct {
	associationID uint64
	writer        vivid.ActorRef
	err           error
	dropCurrent   bool
}

func newEndpointWriter(associationID uint64, session *session, codec *serialization.VividCodec, parentRef vivid.ActorRef) *endpointWriter {
	return &endpointWriter{
		associationID: associationID,
		session:       session,
		codec:         codec,
		parentRef:     parentRef,
	}
}

type endpointWriter struct {
	associationID uint64
	session       *session
	codec         *serialization.VividCodec
	parentRef     vivid.ActorRef
}

func (e *endpointWriter) OnReceive(ctx vivid.ActorContext) {
	switch msg := ctx.Message().(type) {
	case endpointWriterSend:
		e.onSend(ctx, msg)
	}
}

func (e *endpointWriter) onSend(ctx vivid.ActorContext, message endpointWriterSend) {
	data, err := encodeEnvelop(e.codec, message.envelop)
	if err != nil {
		ctx.Tell(e.parentRef, endpointWriterFailed{
			associationID: e.associationID,
			writer:        ctx.Ref(),
			err:           err,
			dropCurrent:   true,
		})
		return
	}

	frame := NewDataFrame(data)
	if err := e.session.WriteFrame(frame); err != nil {
		ctx.Tell(e.parentRef, endpointWriterFailed{
			associationID: e.associationID,
			writer:        ctx.Ref(),
			err:           err,
		})
		return
	}
	ctx.Tell(e.parentRef, endpointWriterAck{associationID: e.associationID, writer: ctx.Ref()})

	bytesLen := uint64(len(frame.Bytes()))
	ctx.Metrics().Counter(metrics.RemotingBytesSentTotalCounter).Add(bytesLen)
	ctx.Metrics().Counter(metrics.RemotingEnvelopSentTotalCounter).Inc()
	ctx.Metrics().Histogram(metrics.RemotingEnvelopSizeBytesHistogram).Observe(float64(len(data)))
}
