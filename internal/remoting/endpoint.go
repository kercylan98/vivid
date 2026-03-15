package remoting

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/serialization"
	"github.com/kercylan98/vivid/pkg/log"
)

var (
	_ vivid.Actor = (*endpoint)(nil)
)

const dialTimeout = 10 * time.Second

const (
	defaultEndpointConnectRetryLimit         = 5
	defaultEndpointConnectRetryInitialDelay  = 500 * time.Millisecond
	defaultEndpointConnectRetryMaxDelay      = 10 * time.Second
	defaultEndpointConnectRetryBackoffFactor = 2
)

type endpointRetryPolicy struct {
	limit        int
	initialDelay time.Duration
	maxDelay     time.Duration
	backoff      float64
}

func newEndpointRetryPolicy(opts *vivid.ActorSystemRemotingOptions) endpointRetryPolicy {
	policy := endpointRetryPolicy{
		limit:        defaultEndpointConnectRetryLimit,
		initialDelay: defaultEndpointConnectRetryInitialDelay,
		maxDelay:     defaultEndpointConnectRetryMaxDelay,
		backoff:      defaultEndpointConnectRetryBackoffFactor,
	}
	if opts == nil {
		return policy
	}
	if opts.ReconnectLimit > 0 {
		policy.limit = opts.ReconnectLimit
	}
	if opts.ReconnectInitialDelay > 0 {
		policy.initialDelay = opts.ReconnectInitialDelay
	}
	if opts.ReconnectMaxDelay > 0 {
		policy.maxDelay = opts.ReconnectMaxDelay
	}
	if opts.ReconnectFactor > 1 {
		policy.backoff = opts.ReconnectFactor
	}
	return policy
}

func (p endpointRetryPolicy) delayFor(retry int) time.Duration {
	if retry <= 0 {
		return p.initialDelay
	}
	delay := p.initialDelay
	for i := 0; i < retry; i++ {
		next := time.Duration(float64(delay) * p.backoff)
		if next <= delay {
			return p.maxDelay
		}
		delay = next
		if delay >= p.maxDelay {
			return p.maxDelay
		}
	}
	return delay
}

type endpointAttachSession struct {
	session *session
}

type endpointRetryConnect struct{}

type endpointConnectCompleted struct {
	attempt uint64
	session *session
	err     error
}

type endpointReaderStopped struct {
	associationID uint64
	reader        vivid.ActorRef
	err           error
	peerClosed    bool
}

func newEndpoint(address string, endpointManager *EndpointManager, codec *serialization.VividCodec, envelopHandler NetworkEnvelopHandler, advertiseAddr string, bufferPolicy endpointBufferPolicy, retryPolicy endpointRetryPolicy, associationPolicy endpointAssociationPolicy) *endpoint {
	return &endpoint{
		address:           address,
		endpointManager:   endpointManager,
		codec:             codec,
		envelopHandler:    envelopHandler,
		advertiseAddr:     advertiseAddr,
		outboundBuffer:    newEndpointOutboundBuffer(bufferPolicy),
		retryPolicy:       retryPolicy,
		associationPolicy: associationPolicy,
	}
}

type endpoint struct {
	address           string
	endpointManager   *EndpointManager
	codec             *serialization.VividCodec
	envelopHandler    NetworkEnvelopHandler
	advertiseAddr     string
	outboundBuffer    *endpointOutboundBuffer
	retryPolicy       endpointRetryPolicy
	associationPolicy endpointAssociationPolicy
	association       *endpointAssociation
	stopping          bool
	connecting        bool
	connectAttempt    uint64
	writing           bool
	retryCount        int
	retryScheduled    bool
	associationSeq    uint64
}

func (e *endpoint) OnReceive(ctx vivid.ActorContext) {
	switch msg := ctx.Message().(type) {
	case *vivid.OnLaunch:
		e.onLaunch(ctx)
	case *vivid.OnKill:
		e.onKill(ctx)
	case *vivid.OnKilled:
		e.onKilled(ctx, msg)
	case vivid.Envelop:
		e.onEnvelop(ctx, msg)
	case endpointAttachSession:
		e.onAttachSession(ctx, msg)
	case endpointRetryConnect:
		e.onRetryConnect(ctx)
	case *vivid.PipeResult:
		e.onPipeResult(ctx, msg)
	case endpointReaderStopped:
		e.onReaderStopped(ctx, msg)
	case endpointWriterAck:
		e.onWriterAck(ctx, msg)
	case endpointWriterFailed:
		e.onWriterFailed(ctx, msg)
	case endpointHeartbeatFailed:
		e.onHeartbeatFailed(ctx, msg)
	}
}

func (e *endpoint) onLaunch(ctx vivid.ActorContext) {
	if e.address == "" {
		ctx.Failed(vivid.ErrorIllegalArgument.WithMessage("endpoint address is empty"))
		return
	}
}

func (e *endpoint) onAttachSession(ctx vivid.ActorContext, message endpointAttachSession) {
	if e.stopping {
		e.closeSession(ctx, message.session, true, "endpoint stopping")
		return
	}
	if message.session == nil {
		ctx.Logger().Warn("endpoint received nil session", log.String("address", e.address))
		return
	}

	if !e.shouldAcceptSession(message.session) {
		ctx.Logger().Debug("endpoint rejected duplicate session",
			log.String("address", e.address),
			log.Any("role", message.session.role))
		e.closeSession(ctx, message.session, true, "duplicate session")
		return
	}

	if err := e.activateSession(ctx, message.session); err != nil {
		ctx.Logger().Warn("endpoint activate attached session failed",
			log.String("address", e.address),
			log.Any("error", err))
		e.scheduleRetry(ctx, err)
		return
	}

	e.trySend(ctx)
}

func (e *endpoint) ensureSession(ctx vivid.ActorContext) {
	if e.stopping {
		return
	}
	if e.association != nil || e.connecting {
		return
	}
	if e.outboundBuffer.Empty() {
		return
	}

	e.connecting = true
	e.connectAttempt++
	attempt := e.connectAttempt
	ctx.Entrust(0, vivid.EntrustTaskFN(func() (vivid.Message, error) {
		conn, err := e.dial()
		if err != nil {
			return &endpointConnectCompleted{
				attempt: attempt,
				err:     err,
			}, nil
		}

		localAddr := e.advertiseAddr
		if localAddr == "" {
			localAddr = conn.LocalAddr().String()
		}
		return &endpointConnectCompleted{
			attempt: attempt,
			session: newSession(e.address, localAddr, conn, sessionRoleOutbound),
		}, nil
	})).PipeTo(ctx.Ref().ToActorRefs())
}

func (e *endpoint) dial() (net.Conn, error) {
	addr := e.address
	dialer := net.Dialer{Timeout: dialTimeout}
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	h := &Handshake{AdvertiseAddr: e.advertiseAddr}
	if err := h.Send(conn); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
		return nil, err
	}
	response := &Handshake{}
	if err := response.Wait(conn); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
		return nil, err
	}
	if response.AdvertiseAddr != "" && response.AdvertiseAddr != addr {
		mismatchErr := vivid.ErrorRemotingHandshake.WithMessage("remote advertise address mismatch")
		if closeErr := conn.Close(); closeErr != nil {
			return nil, errors.Join(mismatchErr, closeErr)
		}
		return nil, mismatchErr
	}
	return conn, nil
}

func (e *endpoint) onEnvelop(ctx vivid.ActorContext, envelop vivid.Envelop) {
	if e.stopping {
		e.envelopHandler.HandleFailedRemotingEnvelop(envelop)
		return
	}
	if err := e.outboundBuffer.Push(envelop); err != nil {
		ctx.Logger().Warn("endpoint outbound buffer full",
			log.String("address", e.address),
			log.Any("error", err))
		e.envelopHandler.HandleFailedRemotingEnvelop(envelop)
		return
	}
	e.trySend(ctx)
}

func (e *endpoint) onRetryConnect(ctx vivid.ActorContext) {
	e.retryScheduled = false
	if e.stopping {
		return
	}
	if e.association != nil || e.outboundBuffer.Empty() {
		return
	}
	e.ensureSession(ctx)
}

func (e *endpoint) onPipeResult(ctx vivid.ActorContext, result *vivid.PipeResult) {
	if e.stopping {
		if result != nil {
			if completed, ok := result.Message.(*endpointConnectCompleted); ok && completed != nil && completed.session != nil {
				e.closeSession(ctx, completed.session, true, "endpoint stopping")
			}
		}
		return
	}
	if result == nil {
		ctx.Logger().Warn("endpoint received nil pipe result", log.String("address", e.address))
		return
	}

	completed, ok := result.Message.(*endpointConnectCompleted)
	if !ok {
		if result.Error != nil {
			ctx.Logger().Warn("endpoint received unexpected pipe error",
				log.String("address", e.address),
				log.Any("error", result.Error))
			if e.connecting {
				e.connecting = false
				e.scheduleRetry(ctx, result.Error)
			}
		}
		return
	}

	e.onConnectCompleted(ctx, completed, result.Error)
}

func (e *endpoint) onConnectCompleted(ctx vivid.ActorContext, completed *endpointConnectCompleted, pipeErr error) {
	if completed == nil {
		ctx.Logger().Warn("endpoint connect completed with nil payload", log.String("address", e.address))
		return
	}
	if completed.attempt != e.connectAttempt {
		if completed.session != nil {
			e.closeSession(ctx, completed.session, true, "stale connect result")
		}
		return
	}

	e.connecting = false
	if pipeErr != nil {
		ctx.Logger().Warn("endpoint connect pipe failed",
			log.String("address", e.address),
			log.Any("error", pipeErr))
		e.scheduleRetry(ctx, pipeErr)
		return
	}
	if completed.err != nil {
		ctx.Logger().Warn("endpoint connect failed",
			log.String("address", e.address),
			log.Any("error", completed.err))
		e.scheduleRetry(ctx, completed.err)
		return
	}
	if completed.session == nil {
		nilSessionErr := vivid.ErrorRemotingHandshake.WithMessage("endpoint connect completed without session")
		ctx.Logger().Warn("endpoint connect returned nil session",
			log.String("address", e.address),
			log.Any("error", nilSessionErr))
		e.scheduleRetry(ctx, nilSessionErr)
		return
	}

	if !e.shouldAcceptSession(completed.session) {
		ctx.Logger().Debug("endpoint rejected completed session",
			log.String("address", e.address),
			log.Any("role", completed.session.role))
		e.closeSession(ctx, completed.session, true, "connect completed with duplicate session")
		return
	}

	if err := e.activateSession(ctx, completed.session); err != nil {
		ctx.Logger().Warn("endpoint activate connected session failed",
			log.String("address", e.address),
			log.Any("error", err))
		e.scheduleRetry(ctx, err)
		return
	}
	e.trySend(ctx)
}

func (e *endpoint) onReaderStopped(ctx vivid.ActorContext, message endpointReaderStopped) {
	if e.stopping {
		return
	}
	if e.association == nil || e.association.id != message.associationID || !e.association.ownsReader(message.reader) {
		return
	}

	if message.peerClosed {
		ctx.Logger().Debug("endpoint peer closed session", log.String("address", e.address))
	} else if message.err != nil {
		ctx.Logger().Warn("endpoint reader stopped",
			log.String("address", e.address),
			log.Any("error", message.err))
	}

	e.association.reader = nil
	e.closeCurrentSession(ctx, false, "reader stopped")
	e.ensureSession(ctx)
}

func (e *endpoint) onWriterAck(ctx vivid.ActorContext, message endpointWriterAck) {
	if e.stopping {
		return
	}
	if e.association == nil || e.association.id != message.associationID || !e.association.ownsWriter(message.writer) {
		return
	}
	if e.outboundBuffer.Empty() {
		e.writing = false
		return
	}
	e.outboundBuffer.Pop()
	e.writing = false
	e.retryCount = 0
	e.trySend(ctx)
}

func (e *endpoint) onWriterFailed(ctx vivid.ActorContext, message endpointWriterFailed) {
	if e.stopping {
		return
	}
	if e.association == nil || e.association.id != message.associationID || !e.association.ownsWriter(message.writer) {
		return
	}

	e.writing = false
	if !e.outboundBuffer.Empty() && message.dropCurrent {
		if current := e.outboundBuffer.Pop(); current != nil {
			e.envelopHandler.HandleFailedRemotingEnvelop(current)
		}
		e.trySend(ctx)
		return
	}

	if message.err != nil {
		ctx.Logger().Warn("endpoint writer failed",
			log.String("address", e.address),
			log.Any("error", message.err))
	}
	e.closeCurrentSession(ctx, false, "writer failed")
	e.scheduleRetry(ctx, message.err)
}

func (e *endpoint) onHeartbeatFailed(ctx vivid.ActorContext, message endpointHeartbeatFailed) {
	if e.stopping {
		return
	}
	if e.association == nil || e.association.id != message.associationID || !e.association.ownsHeartbeat(message.heartbeat) {
		return
	}
	if message.err != nil {
		ctx.Logger().Warn("endpoint heartbeat failed",
			log.String("address", e.address),
			log.Any("error", message.err))
	}
	e.closeCurrentSession(ctx, false, "heartbeat failed")
	e.scheduleRetry(ctx, message.err)
}

func (e *endpoint) onKilled(ctx vivid.ActorContext, message *vivid.OnKilled) {
	if ctx.Ref().Equals(message.Ref) {
		return
	}
	if e.stopping {
		return
	}
	if e.association != nil && e.association.ownsReader(message.Ref) {
		e.writing = false
		e.association.reader = nil
		e.closeCurrentSession(ctx, false, "reader actor killed")
		e.ensureSession(ctx)
	}
	if e.association != nil && e.association.ownsWriter(message.Ref) {
		e.writing = false
		e.association.writer = nil
		e.closeCurrentSession(ctx, false, "writer actor killed")
		e.ensureSession(ctx)
	}
	if e.association != nil && e.association.ownsHeartbeat(message.Ref) {
		e.writing = false
		e.association.heartbeat = nil
		e.closeCurrentSession(ctx, false, "heartbeat actor killed")
		e.ensureSession(ctx)
	}
}

func (e *endpoint) onKill(ctx vivid.ActorContext) {
	e.stopping = true
	e.cancelRetry(ctx)
	e.failPending(ctx)
	// 仅关闭 TCP 会话，不再 Kill 子 Actor。
	// 框架的 doKill 在调用此行为之前已向所有子 Actor 发送了 Kill 消息。
	if e.association != nil {
		e.association.closeSession(ctx, e.address, true, "endpoint closing")
		e.association = nil
	}
	e.endpointManager.unregisterEndpoint(e.address, ctx.Ref())
	ctx.Logger().Debug("endpoint closing", log.String("address", e.address))
}

func (e *endpoint) trySend(ctx vivid.ActorContext) {
	if e.outboundBuffer.Empty() {
		return
	}
	if e.association == nil {
		e.ensureSession(ctx)
		return
	}
	if e.association.writer == nil {
		ctx.Logger().Warn("endpoint has no writer", log.String("address", e.address))
		e.closeCurrentSession(ctx, false, "writer missing")
		e.scheduleRetry(ctx, vivid.ErrorRemotingEndpointSendData.WithMessage("endpoint writer missing"))
		return
	}
	if e.writing {
		return
	}
	current := e.outboundBuffer.Peek()
	if current == nil {
		return
	}
	e.writing = true
	ctx.Tell(e.association.writer, endpointWriterSend{envelop: current})
}

func (e *endpoint) activateSession(ctx vivid.ActorContext, next *session) error {
	e.cancelRetry(ctx)
	e.connecting = false
	e.retryCount = 0
	e.writing = false

	if e.association != nil && e.association.session != next {
		e.closeCurrentSession(ctx, true, "session replaced")
	}

	e.associationSeq++
	association, err := spawnEndpointAssociation(ctx, e.associationSeq, next, e.codec, e.envelopHandler, e.associationPolicy)
	if err != nil {
		e.closeSession(ctx, next, false, "association spawn failed")
		return err
	}
	e.association = association
	return nil
}

func (e *endpoint) shouldAcceptSession(next *session) bool {
	if next == nil {
		return false
	}
	if e.association == nil || e.association.session == nil || e.association.session.isClosed() {
		return true
	}
	if e.association.session.role == next.role {
		return false
	}
	return next.role == e.preferredSessionRole(next)
}

func (e *endpoint) preferredSessionRole(next *session) sessionRole {
	localAddr := e.advertiseAddr
	if next != nil && next.localAddress != "" {
		localAddr = next.localAddress
	}
	if localAddr != "" && e.address != "" && localAddr < e.address {
		return sessionRoleOutbound
	}
	return sessionRoleInbound
}

func (e *endpoint) scheduleRetry(ctx vivid.ActorContext, cause error) {
	if e.outboundBuffer.Empty() {
		return
	}
	if e.retryScheduled {
		return
	}
	if e.retryCount >= e.retryPolicy.limit {
		ctx.Logger().Warn("endpoint retries exhausted",
			log.String("address", e.address),
			log.Any("error", cause),
			log.Any("pending", e.outboundBuffer.Len()))
		e.failPending(ctx)
		ctx.Kill(ctx.Ref(), false, "endpoint retries exhausted")
		return
	}

	delay := e.retryPolicy.delayFor(e.retryCount)
	e.retryCount++
	e.retryScheduled = true
	if err := ctx.Scheduler().Once(ctx.Ref(), delay, endpointRetryConnect{}, vivid.WithSchedulerReference(e.retryScheduleReference())); err != nil {
		e.retryScheduled = false
		ctx.Logger().Warn("endpoint schedule retry failed",
			log.String("address", e.address),
			log.Any("error", err))
		e.failPending(ctx)
		ctx.Kill(ctx.Ref(), false, "endpoint retry scheduling failed")
	}
}

func (e *endpoint) cancelRetry(ctx vivid.ActorContext) {
	if !e.retryScheduled {
		return
	}
	if err := ctx.Scheduler().Cancel(e.retryScheduleReference()); err != nil && !errors.Is(err, vivid.ErrorNotFound) {
		ctx.Logger().Warn("endpoint cancel retry failed",
			log.String("address", e.address),
			log.Any("error", err))
	}
	e.retryScheduled = false
}

func (e *endpoint) retryScheduleReference() string {
	return fmt.Sprintf("endpoint-retry:%s", e.address)
}

func (e *endpoint) failPending(_ vivid.ActorContext) {
	if e.outboundBuffer.Empty() {
		return
	}
	e.outboundBuffer.FailAll(e.envelopHandler)
	e.connecting = false
	e.writing = false
	e.retryCount = 0
	e.retryScheduled = false
}

func (e *endpoint) closeCurrentSession(ctx vivid.ActorContext, graceful bool, reason string) {
	e.writing = false
	if e.association != nil {
		e.association.close(ctx, e.address, graceful, reason)
		e.association = nil
	}
}

func (e *endpoint) closeSession(ctx vivid.ActorContext, target *session, graceful bool, reason string) {
	closeEndpointSession(ctx, e.address, target, graceful, reason)
}
