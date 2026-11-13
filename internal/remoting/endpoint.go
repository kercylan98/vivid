package remoting

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"net"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/serialization"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/metrics"
	"github.com/kercylan98/vivid/pkg/ves"
)

var (
	_ vivid.Actor = (*endpoint)(nil)
)

const dialTimeout = 10 * time.Second

type endpointRetryPolicy struct {
	limit        int           // 重试次数限制
	initialDelay time.Duration // 初始重试延迟
	maxDelay     time.Duration // 最大重试延迟
	backoff      float64       // 重试延迟倍数
	jitter       bool          // 在退避间隔的后半段随机取值
}

func newEndpointRetryPolicy(opts *vivid.ActorSystemRemotingOptions) endpointRetryPolicy {
	defaults := vivid.NewActorSystemRemotingOptions()
	if opts == nil {
		opts = &defaults
	}
	policy := endpointRetryPolicy{
		limit:        defaults.ReconnectLimit,
		initialDelay: defaults.ReconnectInitialDelay,
		maxDelay:     defaults.ReconnectMaxDelay,
		backoff:      defaults.ReconnectFactor,
		jitter:       opts.ReconnectJitter,
	}
	if opts.ReconnectLimit >= 0 {
		policy.limit = opts.ReconnectLimit
	}
	if opts.ReconnectInitialDelay > 0 {
		policy.initialDelay = opts.ReconnectInitialDelay
	}
	if opts.ReconnectMaxDelay > 0 {
		policy.maxDelay = opts.ReconnectMaxDelay
	}
	if opts.ReconnectFactor >= 1 && !math.IsInf(opts.ReconnectFactor, 0) {
		policy.backoff = opts.ReconnectFactor
	}
	return policy
}

func (p endpointRetryPolicy) delayFor(retry int) time.Duration {
	delay := min(p.initialDelay, p.maxDelay)
	if retry > 0 && p.backoff > 1 && delay < p.maxDelay {
		next := float64(delay) * math.Pow(p.backoff, float64(retry))
		// 转换为 Duration 前截断，避免浮点溢出或舍入为负值。
		if next >= float64(p.maxDelay) {
			delay = p.maxDelay
		} else {
			delay = time.Duration(next)
		}
	}
	if p.jitter && delay > 1 {
		lower := delay/2 + delay%2
		return lower + time.Duration(rand.Int64N(int64(delay-lower)+1))
	}
	return delay
}

type endpointAttachSession struct {
	session *session
}

type endpointInboundReady struct {
	session *session
	err     error
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

func newEndpoint(availableSignal <-chan struct{}, address string, endpointManager *EndpointManager, codec *serialization.VividCodec, envelopHandler NetworkEnvelopHandler, advertiseAddr string, bufferPolicy endpointBufferPolicy, retryPolicy endpointRetryPolicy, associationPolicy endpointAssociationPolicy) *endpoint {
	return &endpoint{
		availableSignal:   availableSignal,
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
	availableSignal   <-chan struct{}
	address           string
	endpointManager   *EndpointManager
	codec             *serialization.VividCodec
	envelopHandler    NetworkEnvelopHandler
	advertiseAddr     string
	outboundBuffer    *endpointOutboundBuffer
	retryPolicy       endpointRetryPolicy
	associationPolicy endpointAssociationPolicy
	association       *endpointAssociation
	inboundSession    *session // 已通过仲裁，尚未完成握手响应的入站会话
	fallbackSession   *session // 同时拨号时暂存的非优选入站会话
	stopping          bool
	connecting        bool
	connectAttempt    uint64 // 当前尝试向该端点发起连接尝试的单调递增计数
	connectTask       *pendingConnection
	writing           bool
	retryCount        int  // 当前重试次数
	retryScheduled    bool // 是否已调度重试
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
	case endpointReaderStopped:
		e.onReaderStopped(ctx, msg)
	case endpointWriterAck:
		e.onWriterAck(ctx, msg)
	case endpointWriterFailed:
		e.onWriterFailed(ctx, msg)
	case endpointHeartbeatFailed:
		e.onHeartbeatFailed(ctx, msg)
	case *vivid.PipeResult:
		switch message := msg.Message.(type) {
		case *endpointConnectCompleted:
			e.onConnectCompleted(ctx, message, msg.Error)
		case *endpointInboundReady:
			e.onInboundReady(ctx, message, msg.Error)
		default:
			e.onPipeUnexpectedResult(ctx, message, msg.Error)
		}
	}
}

func (e *endpoint) onPipeUnexpectedResult(ctx vivid.ActorContext, message vivid.Message, pipeErr error) {
	if pipeErr != nil {
		ctx.Logger().Warn("endpoint received unexpected pipe error", log.String("address", e.address), log.String("type", fmt.Sprintf("%T", message)), log.Any("error", pipeErr))
		if e.connecting {
			if e.connectTask != nil {
				e.connectTask.close()
				e.connectTask = nil
			}
			e.connecting = false
			e.onConnectFailed(ctx, pipeErr)
		}
		return
	}
}

func (e *endpoint) onLaunch(ctx vivid.ActorContext) {
	if e.address == "" {
		ctx.Failed(vivid.ErrorIllegalArgument.WithMessage("endpoint address is empty"))
		return
	}
	ctx.Metrics().Gauge(metrics.RemotingOutboundEndpointsGauge).Inc()
}

func (e *endpoint) onAttachSession(ctx vivid.ActorContext, message endpointAttachSession) {
	if e.stopping {
		e.closeSession(ctx, message.session, false, "endpoint stopping")
		return
	}
	if message.session == nil {
		ctx.Logger().Warn("endpoint received nil session", log.String("address", e.address))
		return
	}

	if !e.shouldAcceptSession(message.session) {
		if e.connecting && e.association == nil && e.inboundSession == nil && e.fallbackSession == nil &&
			message.session.role == sessionRoleInbound && e.preferredSessionRole(message.session) == sessionRoleOutbound {
			// 等优选握手完成后再关闭，避免对端先收到 EOF 而尚未接管优选连接。
			e.fallbackSession = message.session
			return
		}
		ctx.Logger().Debug("endpoint rejected duplicate session",
			log.String("address", e.address),
			log.Any("role", message.session.role))
		e.closeSession(ctx, message.session, false, "duplicate session")
		return
	}

	// 先保留入站会话并作废并行拨号，再异步回复握手；未就绪时不发送业务数据。
	e.cancelRetry(ctx)
	e.inboundSession = message.session
	e.connectAttempt++
	if e.connectTask != nil {
		e.connectTask.close()
		e.connectTask = nil
	}
	e.connecting = false
	ctx.Entrust(0, vivid.EntrustTaskFN(func() (vivid.Message, error) {
		response := &Handshake{AdvertiseAddr: message.session.localAddress}
		return &endpointInboundReady{session: message.session, err: response.Send(message.session.Conn())}, nil
	})).PipeTo(ctx.Ref().ToActorRefs())
}

func (e *endpoint) onInboundReady(ctx vivid.ActorContext, message *endpointInboundReady, pipeErr error) {
	if e.stopping || e.inboundSession != message.session {
		e.closeSession(ctx, message.session, false, "stale inbound handshake")
		return
	}
	e.inboundSession = nil
	if err := errors.Join(message.err, pipeErr); err != nil {
		ctx.Logger().Warn("endpoint inbound handshake failed", log.String("address", e.address), log.Any("error", err))
		e.closeSession(ctx, message.session, false, "inbound handshake failed")
		e.scheduleRetry(ctx, err)
		return
	}
	if err := e.activateSession(ctx, message.session); err != nil {
		ctx.Logger().Warn("endpoint activate attached session failed", log.String("address", e.address), log.Any("error", err))
		e.scheduleRetry(ctx, err)
		return
	}
	ctx.EventStream().Publish(ctx, ves.RemotingInboundConnectionEstablishedEvent{
		PeerAddress:  e.address,
		LocalAddress: message.session.localAddress,
	})
	e.trySend(ctx)
}

func (e *endpoint) ensureSession(ctx vivid.ActorContext) {
	if e.stopping {
		return
	}
	if e.association != nil || e.connecting || e.inboundSession != nil || e.retryScheduled {
		return
	}
	if e.outboundBuffer.Empty() {
		return
	}

	e.connecting = true
	e.connectAttempt++
	attempt := e.connectAttempt
	task := e.endpointManager.newPendingConnection(dialTimeout + handshakeTimeout)
	e.connectTask = task
	ctx.Entrust(0, vivid.EntrustTaskFN(func() (vivid.Message, error) {
		conn, err := e.dial(task)
		if err != nil {
			task.close()
			return &endpointConnectCompleted{
				attempt: attempt,
				err:     err,
			}, nil
		}

		localAddr := e.advertiseAddr
		if localAddr == "" {
			localAddr = conn.LocalAddr().String()
		}
		session := newSession(e.address, localAddr, conn, sessionRoleOutbound)
		session.pending = task
		return &endpointConnectCompleted{
			attempt: attempt,
			session: session,
		}, nil
	})).PipeTo(ctx.Ref().ToActorRefs())
}

func (e *endpoint) dial(task *pendingConnection) (net.Conn, error) {
	addr := e.address
	dialer := net.Dialer{Timeout: dialTimeout}
	conn, err := dialer.DialContext(task.ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	if !task.attach(conn) {
		return nil, task.ctx.Err()
	}
	if config := e.associationPolicy.tlsConfig; config != nil {
		conn, err = handshakeTLS(task.ctx, conn, addr, config)
		if err != nil {
			return nil, err
		}
	}
	h := &Handshake{AdvertiseAddr: e.advertiseAddr}
	if err := h.Send(conn); err != nil {
		if closeErr := closeConnection(conn); closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
		return nil, err
	}
	response := &Handshake{}
	if err := response.Wait(conn); err != nil {
		if closeErr := closeConnection(conn); closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
		return nil, err
	}
	if response.AdvertiseAddr != "" && response.AdvertiseAddr != addr {
		mismatchErr := vivid.ErrorRemotingHandshake.WithMessage("remote advertise address mismatch")
		if closeErr := closeConnection(conn); closeErr != nil {
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
		ctx.EventStream().Publish(ctx, ves.RemotingEnvelopSendFailedEvent{
			TargetAddress: e.address,
			Error:         err,
		})
		e.envelopHandler.HandleFailedRemotingEnvelop(envelop)
		return
	}
	ctx.Metrics().Gauge(metrics.RemotingPendingEnvelopsGauge).Inc()
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

func (e *endpoint) onConnectCompleted(ctx vivid.ActorContext, completed *endpointConnectCompleted, pipeErr error) {
	// 停止中，关闭连接
	if e.stopping {
		e.closeSession(ctx, completed.session, false, "endpoint stopping")
		return
	}

	// 连接结果已过期
	if completed.attempt != e.connectAttempt {
		e.closeSession(ctx, completed.session, false, "stale connect result")
		return
	}

	e.connecting = false
	e.connectTask = nil

	if pipeErr != nil {
		e.closeSession(ctx, completed.session, false, "connect task failed")
		e.onConnectFailed(ctx, pipeErr)
		return
	}

	if completed.err != nil {
		e.onConnectFailed(ctx, completed.err)
		return
	}

	if completed.session == nil {
		nilSessionErr := vivid.ErrorRemotingHandshake.WithMessage("endpoint connect completed without session")
		e.onConnectFailed(ctx, nilSessionErr)
		return
	}

	// 已收到优选连接的握手响应，对端必已完成入站接管，才能释放备用连接。
	e.closeFallbackSession(ctx, "outbound handshake completed")
	if !e.shouldAcceptSession(completed.session) {
		e.closeSession(ctx, completed.session, false, "connect completed with duplicate session")
		return
	}

	if err := e.activateSession(ctx, completed.session); err != nil {
		e.scheduleRetry(ctx, err)
		return
	}

	ctx.EventStream().Publish(ctx, ves.RemotingOutboundConnectionEstablishedEvent{Address: e.address})
	e.trySend(ctx)
}

func (e *endpoint) onConnectFailed(ctx vivid.ActorContext, cause error) {
	if e.fallbackSession != nil {
		// 复用已经接入的连接，不增加拨号次数或消耗额外重试预算。
		next := e.fallbackSession
		e.fallbackSession = nil
		e.onAttachSession(ctx, endpointAttachSession{session: next})
		return
	}
	e.scheduleRetry(ctx, cause)
}

func (e *endpoint) closeFallbackSession(ctx vivid.ActorContext, reason string) {
	if e.fallbackSession != nil {
		e.closeSession(ctx, e.fallbackSession, false, reason)
		e.fallbackSession = nil
	}
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
	ctx.EventStream().Publish(ctx, ves.RemotingConnectionClosedEvent{
		Address:    e.address,
		PeerClosed: message.peerClosed,
		Error:      message.err,
	})
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
	ctx.Metrics().Gauge(metrics.RemotingPendingEnvelopsGauge).Dec()
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
			ctx.Metrics().Gauge(metrics.RemotingPendingEnvelopsGauge).Dec()
			ctx.EventStream().Publish(ctx, ves.RemotingEnvelopSendFailedEvent{
				TargetAddress: e.address,
				Error:         message.err,
			})
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
	ctx.EventStream().Publish(ctx, ves.RemotingConnectionClosedEvent{
		Address:    e.address,
		PeerClosed: false,
		Error:      message.err,
	})
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
	ctx.EventStream().Publish(ctx, ves.RemotingConnectionClosedEvent{
		Address:    e.address,
		PeerClosed: false,
		Error:      message.err,
	})
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
	if e.connectTask != nil {
		e.connectTask.close()
		e.connectTask = nil
	}
	ctx.Metrics().Gauge(metrics.RemotingOutboundEndpointsGauge).Dec()
	e.cancelRetry(ctx)
	e.failPending(ctx)
	if e.inboundSession != nil {
		e.closeSession(ctx, e.inboundSession, false, "endpoint closing")
		e.inboundSession = nil
	}
	e.closeFallbackSession(ctx, "endpoint closing")
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
	if next.pending != nil && !next.pending.release() {
		e.closeSession(ctx, next, false, "connection task canceled")
		return vivid.ErrorRemotingHandshake.WithMessage("connection task canceled")
	}
	e.cancelRetry(ctx)
	e.connecting = false
	e.retryCount = 0
	e.writing = false

	if e.association != nil && e.association.session != next {
		e.closeCurrentSession(ctx, true, "session replaced")
	}

	e.associationSeq++
	association, err := spawnEndpointAssociation(ctx, e.availableSignal, e.associationSeq, next, e.codec, e.envelopHandler, e.associationPolicy)
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
	if e.inboundSession != nil {
		return false
	}
	// 健康会话一旦启用就不再替换，写入 TCP 并不能证明对端已读取数据。
	if e.association != nil && e.association.session != nil && !e.association.session.isClosed() {
		return false
	}
	// 双方同时拨号时，在握手确认前按地址选定同一条连接。
	return next.role != sessionRoleInbound || !e.connecting || e.preferredSessionRole(next) == sessionRoleInbound
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
	ctx.EventStream().Publish(ctx, ves.RemotingConnectionFailedEvent{
		Address: e.address,
		Error:   cause,
	})
	if e.retryCount >= e.retryPolicy.limit {
		ctx.Logger().Warn("endpoint retries exhausted", log.String("address", e.address), log.Any("error", cause), log.Any("pending", e.outboundBuffer.Len()))
		e.failPending(ctx)
		ctx.Kill(ctx.Ref(), false, "endpoint retries exhausted")
		return
	}

	delay := e.retryPolicy.delayFor(e.retryCount)
	e.retryCount++
	e.retryScheduled = true
	if err := ctx.Scheduler().Once(ctx.Ref(), delay, endpointRetryConnect{}, vivid.WithSchedulerReference(e.retryScheduleReference())); err != nil {
		e.retryScheduled = false
		ctx.Logger().Warn("endpoint schedule retry failed", log.String("address", e.address), log.Any("error", err))
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

func (e *endpoint) failPending(ctx vivid.ActorContext) {
	if e.outboundBuffer.Empty() {
		return
	}
	n := e.outboundBuffer.Len()
	if n > 0 {
		ctx.Metrics().Gauge(metrics.RemotingPendingEnvelopsGauge).Add(-int64(n))
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
