package remoting

import (
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/serialization"
	"github.com/kercylan98/vivid/pkg/log"
)

type endpointAssociation struct {
	id        uint64
	session   *session
	reader    vivid.ActorRef
	writer    vivid.ActorRef
	heartbeat vivid.ActorRef
}

type endpointAssociationPolicy struct {
	readTimeout       time.Duration
	heartbeatInterval time.Duration
	readFailedHandler vivid.ActorSystemRemotingConnectionReadFailedHandler
}

const (
	defaultAssociationReadTimeout       = 30 * time.Second
	defaultAssociationHeartbeatInterval = 10 * time.Second
)

func newEndpointAssociationPolicy(opts *vivid.ActorSystemRemotingOptions) endpointAssociationPolicy {
	policy := endpointAssociationPolicy{
		readTimeout:       defaultAssociationReadTimeout,
		heartbeatInterval: defaultAssociationHeartbeatInterval,
	}
	if opts == nil {
		return policy
	}
	if opts.ReadTimeout > 0 {
		policy.readTimeout = opts.ReadTimeout
	}
	if opts.HeartbeatInterval >= 0 {
		policy.heartbeatInterval = opts.HeartbeatInterval
	}
	policy.readFailedHandler = opts.ConnectionReadFailedHandler
	return policy
}

func (a *endpointAssociation) ownsReader(ref vivid.ActorRef) bool {
	return a != nil && a.reader != nil && a.reader.Equals(ref)
}

func (a *endpointAssociation) ownsWriter(ref vivid.ActorRef) bool {
	return a != nil && a.writer != nil && a.writer.Equals(ref)
}

func (a *endpointAssociation) ownsHeartbeat(ref vivid.ActorRef) bool {
	return a != nil && a.heartbeat != nil && a.heartbeat.Equals(ref)
}

func (a *endpointAssociation) close(ctx vivid.ActorContext, address string, graceful bool, reason string) {
	if a == nil {
		return
	}
	if a.reader != nil {
		ctx.Kill(a.reader, false, reason)
	}
	if a.writer != nil {
		ctx.Kill(a.writer, false, reason)
	}
	if a.heartbeat != nil {
		ctx.Kill(a.heartbeat, false, reason)
	}
	if a.session == nil {
		return
	}
	closeEndpointSession(ctx, address, a.session, graceful, reason)
}

func closeEndpointSession(ctx vivid.ActorContext, address string, target *session, graceful bool, reason string) {
	if target == nil {
		return
	}
	if graceful {
		if err := target.WriteFrame(NewCloseFrame()); err != nil &&
			!errors.Is(err, net.ErrClosed) &&
			!errors.Is(err, io.ErrClosedPipe) {
			ctx.Logger().Warn("endpoint close frame write failed",
				log.String("address", address),
				log.String("reason", reason),
				log.Any("error", err))
		}
	}
	if err := target.Close(); err != nil {
		ctx.Logger().Warn("endpoint session close failed",
			log.String("address", address),
			log.String("reason", reason),
			log.Any("error", err))
	}
}

func spawnEndpointAssociation(ctx vivid.ActorContext, id uint64, session *session, codec *serialization.VividCodec, envelopHandler NetworkEnvelopHandler, policy endpointAssociationPolicy) (*endpointAssociation, error) {
	readerName := associationActorName("reader", id)
	readerRef, err := ctx.ActorOf(newEndpointReader(id, session, codec, envelopHandler, ctx.Ref(), policy.readTimeout, policy.readFailedHandler), vivid.WithActorName(readerName))
	if err != nil {
		return nil, vivid.ErrorActorSpawnFailed.With(err)
	}

	writerName := associationActorName("writer", id)
	writerRef, err := ctx.ActorOf(newEndpointWriter(id, session, codec, ctx.Ref()), vivid.WithActorName(writerName))
	if err != nil {
		ctx.Kill(readerRef, false, "writer spawn failed")
		return nil, vivid.ErrorActorSpawnFailed.With(err)
	}

	association := &endpointAssociation{
		id:      id,
		session: session,
		reader:  readerRef,
		writer:  writerRef,
	}
	if policy.heartbeatInterval > 0 {
		heartbeatName := associationActorName("heartbeat", id)
		heartbeatRef, err := ctx.ActorOf(newEndpointHeartbeat(id, session, policy.heartbeatInterval, ctx.Ref()), vivid.WithActorName(heartbeatName))
		if err != nil {
			association.close(ctx, session.address, false, "heartbeat spawn failed")
			return nil, vivid.ErrorActorSpawnFailed.With(err)
		}
		association.heartbeat = heartbeatRef
	}
	return association, nil
}

func associationActorName(kind string, id uint64) string {
	return fmt.Sprintf("%s-%d", kind, id)
}
