package remoting

import (
	"time"

	"github.com/kercylan98/vivid"
)

var (
	_ vivid.Actor = (*endpointHeartbeat)(nil)
)

type endpointHeartbeatFailed struct {
	associationID uint64
	heartbeat     vivid.ActorRef
	err           error
}

type endpointHeartbeatTick struct{}

func newEndpointHeartbeat(associationID uint64, session *session, interval time.Duration, parentRef vivid.ActorRef) *endpointHeartbeat {
	return &endpointHeartbeat{
		associationID: associationID,
		session:       session,
		interval:      interval,
		parentRef:     parentRef,
	}
}

type endpointHeartbeat struct {
	associationID uint64
	session       *session
	interval      time.Duration
	parentRef     vivid.ActorRef
}

func (e *endpointHeartbeat) OnReceive(ctx vivid.ActorContext) {
	switch ctx.Message().(type) {
	case *vivid.OnLaunch:
		e.scheduleNext(ctx)
	case endpointHeartbeatTick:
		e.onTick(ctx)
	}
}

func (e *endpointHeartbeat) onTick(ctx vivid.ActorContext) {
	if err := e.session.Write(heartbeatFrameBytes); err != nil {
		ctx.Tell(e.parentRef, endpointHeartbeatFailed{
			associationID: e.associationID,
			heartbeat:     ctx.Ref(),
			err:           err,
		})
		return
	}
	e.scheduleNext(ctx)
}

func (e *endpointHeartbeat) scheduleNext(ctx vivid.ActorContext) {
	if e.interval <= 0 {
		return
	}
	if err := ctx.Scheduler().Once(ctx.Ref(), e.interval, endpointHeartbeatTick{}); err != nil {
		ctx.Tell(e.parentRef, endpointHeartbeatFailed{
			associationID: e.associationID,
			heartbeat:     ctx.Ref(),
			err:           err,
		})
	}
}
