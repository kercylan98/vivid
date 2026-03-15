package remoting

import (
	"bufio"
	"io"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/serialization"
	"github.com/kercylan98/vivid/pkg/log"
)

var (
	_ vivid.Actor = (*endpointReader)(nil)
)

func newEndpointReader(associationID uint64, session *session, codec *serialization.VividCodec, envelopHandler NetworkEnvelopHandler, parentRef vivid.ActorRef, readTimeout time.Duration) *endpointReader {
	return &endpointReader{
		associationID:  associationID,
		session:        session,
		codec:          codec,
		envelopHandler: envelopHandler,
		parentRef:      parentRef,
		readTimeout:    readTimeout,
	}
}

type endpointReader struct {
	associationID  uint64
	session        *session
	codec          *serialization.VividCodec
	envelopHandler NetworkEnvelopHandler
	parentRef      vivid.ActorRef
	reader         *bufio.Reader
	header         []byte
	readTimeout    time.Duration
}

type endpointReadFrame struct{}

func (e *endpointReader) OnReceive(ctx vivid.ActorContext) {
	switch ctx.Message().(type) {
	case *vivid.OnLaunch:
		e.onLaunch(ctx)
	case endpointReadFrame:
		e.onReadFrame(ctx)
	}
}

func (e *endpointReader) onLaunch(ctx vivid.ActorContext) {
	conn := e.session.Conn()
	if conn == nil {
		if !e.session.isClosed() {
			ctx.Tell(e.parentRef, endpointReaderStopped{
				associationID: e.associationID,
				reader:        ctx.Ref(),
				err:           io.ErrClosedPipe,
			})
		}
		return
	}
	e.reader = bufio.NewReader(conn)
	e.header = make([]byte, frameHeaderSize)
	ctx.Tell(ctx.Ref(), endpointReadFrame{})
}

func (e *endpointReader) onReadFrame(ctx vivid.ActorContext) {
	conn := e.session.Conn()
	if conn == nil {
		if !e.session.isClosed() {
			ctx.Tell(e.parentRef, endpointReaderStopped{
				associationID: e.associationID,
				reader:        ctx.Ref(),
				err:           io.ErrClosedPipe,
			})
		}
		return
	}
	if e.readTimeout > 0 {
		if err := conn.SetReadDeadline(time.Now().Add(e.readTimeout)); err != nil {
			if !e.session.isClosed() {
				ctx.Tell(e.parentRef, endpointReaderStopped{
					associationID: e.associationID,
					reader:        ctx.Ref(),
					err:           err,
				})
			}
			return
		}
	}
	frame, err := ReadFrame(e.reader, e.header, FrameReadLimits{
		MaxControlLen: maxFrameControlLen,
		MaxDataLen:    maxFrameDataLen,
	})
	if err != nil {
		// 如果 session 已被父 Actor 主动关闭，无需通知父 Actor（它已在关闭流程中），
		// 也无需自杀（框架会通过 Kill 消息处理）。
		if e.session.isClosed() {
			return
		}
		if err == io.EOF {
			ctx.Tell(e.parentRef, endpointReaderStopped{
				associationID: e.associationID,
				reader:        ctx.Ref(),
				peerClosed:    true,
			})
			return
		}
		ctx.Logger().Warn("endpoint read frame failed", log.String("address", e.session.address), log.Any("error", err))
		ctx.Tell(e.parentRef, endpointReaderStopped{
			associationID: e.associationID,
			reader:        ctx.Ref(),
			err:           err,
		})
		return
	}

	switch frame.Type {
	case FrameCtrlData:
		if len(frame.Data) == 0 {
			ctx.Tell(ctx.Ref(), endpointReadFrame{})
			return
		}
		system, sender, receiver, messageInstance, decodeErr := decodeEnvelop(e.codec, frame.Data)
		if decodeErr != nil {
			ctx.Logger().Warn("endpoint decode envelop failed", log.String("address", e.session.address), log.Any("error", decodeErr))
			ctx.Tell(e.parentRef, endpointReaderStopped{
				associationID: e.associationID,
				reader:        ctx.Ref(),
				err:           decodeErr,
			})
			return
		}
		if handleErr := e.envelopHandler.HandleRemotingEnvelop(system, sender, receiver, messageInstance); handleErr != nil {
			ctx.Logger().Warn("handle remoting envelop failed", log.String("address", e.session.address), log.Any("error", handleErr))
		}
	case FrameCtrlClose:
		if !e.session.isClosed() {
			ctx.Tell(e.parentRef, endpointReaderStopped{
				associationID: e.associationID,
				reader:        ctx.Ref(),
				peerClosed:    true,
			})
		}
		return
	case FrameCtrlHeartbeat, FrameCtrlHandshake:
	default:
		ctx.Logger().Warn("endpoint received unknown frame type", log.String("address", e.session.address), log.Any("ctrl_type", frame.Type))
	}

	if ctx.Active() {
		ctx.Tell(ctx.Ref(), endpointReadFrame{})
	}
}
