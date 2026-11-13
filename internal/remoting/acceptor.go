package remoting

import (
	"errors"
	"net"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/serialization"
	"github.com/kercylan98/vivid/pkg/log"
)

var (
	_ vivid.Actor = (*acceptor)(nil)
)

const maxPendingHandshakes = 64

func newAcceptor(availableSignal <-chan struct{}, listener *listener, endpointManager *EndpointManager, advertiseAddr string, codec *serialization.VividCodec, envelopHandler NetworkEnvelopHandler, bufferPolicy endpointBufferPolicy, retryPolicy endpointRetryPolicy, associationPolicy endpointAssociationPolicy) *acceptor {
	return &acceptor{
		availableSignal:   availableSignal,
		listener:          listener,
		endpointManager:   endpointManager,
		advertiseAddr:     advertiseAddr,
		codec:             codec,
		envelopHandler:    envelopHandler,
		bufferPolicy:      bufferPolicy,
		retryPolicy:       retryPolicy,
		associationPolicy: associationPolicy,
	}
}

type acceptor struct {
	availableSignal   <-chan struct{}
	listener          *listener
	endpointManager   *EndpointManager
	advertiseAddr     string
	codec             *serialization.VividCodec
	envelopHandler    NetworkEnvelopHandler
	bufferPolicy      endpointBufferPolicy
	retryPolicy       endpointRetryPolicy
	associationPolicy endpointAssociationPolicy
	stopping          bool
	accepting         bool
	acceptAttempt     uint64
	acceptTask        *pendingConnection
	handshakeSeq      uint64
	pendingHandshakes map[uint64]*pendingConnection
}

type acceptConnection struct{}

type acceptCompleted struct {
	attempt uint64
	conn    net.Conn
	pending *pendingConnection
	err     error
}

type acceptHandshakeCompleted struct {
	seq      uint64
	conn     net.Conn
	peerAddr string
	err      error
}

func (a *acceptor) OnReceive(ctx vivid.ActorContext) {
	switch ctx.Message().(type) {
	case *vivid.OnLaunch:
		a.onLaunch(ctx)
	case acceptConnection:
		a.onAccept(ctx)
	case *vivid.OnKill:
		a.onKill(ctx)
	case *vivid.PipeResult:
		a.onPipeResult(ctx, ctx.Message().(*vivid.PipeResult))
	}
}

func (a *acceptor) onLaunch(ctx vivid.ActorContext) {
	if a.pendingHandshakes == nil {
		a.pendingHandshakes = make(map[uint64]*pendingConnection)
	}
	ctx.Tell(ctx.Ref(), acceptConnection{})
}

func (a *acceptor) onAccept(ctx vivid.ActorContext) {
	if a.stopping || a.accepting || len(a.pendingHandshakes) >= maxPendingHandshakes {
		return
	}
	a.accepting = true
	a.acceptAttempt++
	attempt := a.acceptAttempt
	task := a.endpointManager.newPendingConnection(0)
	a.acceptTask = task
	ctx.Entrust(0, vivid.EntrustTaskFN(func() (vivid.Message, error) {
		conn, err := a.listener.Accept()
		if err == nil {
			if !task.attach(conn) {
				return &acceptCompleted{attempt: attempt, err: net.ErrClosed, pending: task}, nil
			}
			task.startTimeout(handshakeTimeout)
		} else {
			task.close()
		}
		return &acceptCompleted{
			attempt: attempt,
			conn:    conn,
			pending: task,
			err:     err,
		}, nil
	})).PipeTo(ctx.Ref().ToActorRefs())
}

func (a *acceptor) onKill(ctx vivid.ActorContext) {
	a.stopping = true
	if a.acceptTask != nil {
		a.acceptTask.close()
		a.acceptTask = nil
	}
	if a.listener != nil {
		if err := a.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			ctx.Logger().Warn("acceptor listener close failed", log.Any("error", err))
		}
	}
	for seq, task := range a.pendingHandshakes {
		task.close()
		delete(a.pendingHandshakes, seq)
	}
}

func (a *acceptor) onPipeResult(ctx vivid.ActorContext, result *vivid.PipeResult) {
	if result == nil {
		return
	}
	switch message := result.Message.(type) {
	case *acceptCompleted:
		a.onAcceptCompleted(ctx, message, result.Error)
	case *acceptHandshakeCompleted:
		a.onHandshakeCompleted(ctx, message, result.Error)
	}
}

func (a *acceptor) onAcceptCompleted(ctx vivid.ActorContext, message *acceptCompleted, pipeErr error) {
	if message == nil {
		a.accepting = false
		return
	}
	if a.stopping || message.attempt != a.acceptAttempt {
		message.pending.close()
		return
	}
	a.accepting = false
	a.acceptTask = nil
	if pipeErr != nil {
		message.pending.close()
		if a.stopping {
			return
		}
		ctx.Failed(vivid.ErrorRemotingAccept.With(pipeErr))
		return
	}
	if message.err != nil {
		message.pending.close()
		if errors.Is(message.err, net.ErrClosed) {
			return
		}
		ctx.Failed(vivid.ErrorRemotingAccept.With(message.err))
		return
	}
	if message.conn == nil {
		if !a.stopping && ctx.Available() {
			ctx.Tell(ctx.Ref(), acceptConnection{})
		}
		return
	}
	a.startHandshake(ctx, message.conn, message.pending)
	if !a.stopping && ctx.Available() {
		ctx.Tell(ctx.Ref(), acceptConnection{})
	}
}

func (a *acceptor) startHandshake(ctx vivid.ActorContext, conn net.Conn, task *pendingConnection) {
	if conn == nil {
		return
	}
	a.handshakeSeq++
	seq := a.handshakeSeq
	a.pendingHandshakes[seq] = task
	ctx.Entrust(0, vivid.EntrustTaskFN(func() (vivid.Message, error) {
		peerAddr, err := a.doHandshake(conn)
		if err != nil {
			task.close()
		}
		return &acceptHandshakeCompleted{
			seq:      seq,
			conn:     conn,
			peerAddr: peerAddr,
			err:      err,
		}, nil
	})).PipeTo(ctx.Ref().ToActorRefs())
}

func (a *acceptor) onHandshakeCompleted(ctx vivid.ActorContext, message *acceptHandshakeCompleted, pipeErr error) {
	if message == nil {
		return
	}
	task, ok := a.pendingHandshakes[message.seq]
	if !ok {
		if message.conn != nil {
			_ = closeConnection(message.conn)
		}
		return
	}
	delete(a.pendingHandshakes, message.seq)
	if !a.stopping {
		ctx.Tell(ctx.Ref(), acceptConnection{})
	}
	conn := message.conn
	if pipeErr != nil {
		task.close()
		if !a.stopping {
			ctx.Logger().Warn("acceptor handshake task failed", log.Any("error", pipeErr))
		}
		return
	}
	if message.err != nil {
		if !a.stopping {
			ctx.Logger().Warn("acceptor handshake failed", log.Any("error", message.err))
		}
		task.close()
		return
	}
	if a.stopping || a.endpointManager.Stopped() {
		task.close()
		return
	}

	localAddr := a.advertiseAddr
	if localAddr == "" {
		localAddr = conn.LocalAddr().String()
	}
	session := newSession(message.peerAddr, localAddr, conn, sessionRoleInbound)
	session.pending = task
	endpointRef, ensureErr := a.endpointManager.ensureEndpoint(ctx, message.peerAddr, func(address string) (vivid.ActorRef, error) {
		endpointRef, err := ctx.ActorOf(newEndpoint(a.availableSignal, address, a.endpointManager, a.codec, a.envelopHandler, a.advertiseAddr, a.bufferPolicy, a.retryPolicy, a.associationPolicy))
		if err != nil {
			return nil, err
		}
		return endpointRef, nil
	})
	if ensureErr != nil {
		spawnErr := vivid.ErrorRemotingEndpointUnableToCreate.With(ensureErr)
		if closeErr := session.Close(); closeErr != nil {
			spawnErr = spawnErr.With(closeErr)
		}
		ctx.Logger().Error("failed to ensure endpoint", log.Any("error", spawnErr))
		return
	}
	ctx.Tell(endpointRef, endpointAttachSession{session: session})
}

func (a *acceptor) doHandshake(conn net.Conn) (string, error) {
	request := &Handshake{}
	if err := request.Wait(conn); err != nil {
		if closeErr := closeConnection(conn); closeErr != nil {
			return "", errors.Join(err, closeErr)
		}
		return "", err
	}
	peerAddr := request.AdvertiseAddr
	if peerAddr == "" {
		peerAddr = conn.RemoteAddr().String()
	}
	// 由 Endpoint 完成连接仲裁后再回复握手，避免对端提前发送的数据随连接替换丢失。
	return peerAddr, nil
}
