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

func newAcceptor(listener *listener, endpointManager *EndpointManager, advertiseAddr string, codec *serialization.VividCodec, envelopHandler NetworkEnvelopHandler, bufferPolicy endpointBufferPolicy, retryPolicy endpointRetryPolicy, associationPolicy endpointAssociationPolicy) *acceptor {
	return &acceptor{
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
	handshakeSeq      uint64
	pendingHandshakes map[uint64]net.Conn
}

type acceptConnection struct{}

type acceptCompleted struct {
	attempt uint64
	conn    net.Conn
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
		a.pendingHandshakes = make(map[uint64]net.Conn)
	}
	ctx.Tell(ctx.Ref(), acceptConnection{})
}

func (a *acceptor) onAccept(ctx vivid.ActorContext) {
	if a.stopping || a.accepting {
		return
	}
	a.accepting = true
	a.acceptAttempt++
	attempt := a.acceptAttempt
	ctx.Entrust(0, vivid.EntrustTaskFN(func() (vivid.Message, error) {
		conn, err := a.listener.Accept()
		return &acceptCompleted{
			attempt: attempt,
			conn:    conn,
			err:     err,
		}, nil
	})).PipeTo(ctx.Ref().ToActorRefs())
}

func (a *acceptor) onKill(ctx vivid.ActorContext) {
	a.stopping = true
	if a.listener != nil {
		if err := a.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			ctx.Logger().Warn("acceptor listener close failed", log.Any("error", err))
		}
	}
	for seq, conn := range a.pendingHandshakes {
		if conn == nil {
			delete(a.pendingHandshakes, seq)
			continue
		}
		if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			ctx.Logger().Warn("acceptor pending handshake close failed", log.Any("error", err))
		}
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
	if message.attempt != a.acceptAttempt {
		if message.conn != nil {
			_ = message.conn.Close()
		}
		return
	}
	a.accepting = false
	if pipeErr != nil {
		if a.stopping {
			return
		}
		ctx.Failed(vivid.ErrorRemotingListenerAcceptFailed.With(pipeErr))
		return
	}
	if message.err != nil {
		if errors.Is(message.err, net.ErrClosed) {
			return
		}
		ctx.Failed(vivid.ErrorRemotingListenerAcceptFailed.With(message.err))
		return
	}
	if message.conn == nil {
		if !a.stopping && ctx.Active() {
			ctx.Tell(ctx.Ref(), acceptConnection{})
		}
		return
	}
	a.startHandshake(ctx, message.conn)
	if !a.stopping && ctx.Active() {
		ctx.Tell(ctx.Ref(), acceptConnection{})
	}
}

func (a *acceptor) startHandshake(ctx vivid.ActorContext, conn net.Conn) {
	if conn == nil {
		return
	}
	a.handshakeSeq++
	seq := a.handshakeSeq
	a.pendingHandshakes[seq] = conn
	ctx.Entrust(0, vivid.EntrustTaskFN(func() (vivid.Message, error) {
		peerAddr, err := a.doHandshake(conn)
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
	conn, ok := a.pendingHandshakes[message.seq]
	if ok {
		delete(a.pendingHandshakes, message.seq)
	} else if message.conn != nil {
		conn = message.conn
	}
	if conn == nil {
		return
	}
	if pipeErr != nil {
		_ = conn.Close()
		if !a.stopping {
			ctx.Logger().Warn("acceptor handshake task failed", log.Any("error", pipeErr))
		}
		return
	}
	if message.err != nil {
		if !a.stopping {
			ctx.Logger().Warn("acceptor handshake failed", log.Any("error", message.err))
		}
		_ = conn.Close()
		return
	}
	if a.stopping || a.endpointManager.Stopped() {
		_ = conn.Close()
		return
	}

	localAddr := a.advertiseAddr
	if localAddr == "" {
		localAddr = conn.LocalAddr().String()
	}
	session := newSession(message.peerAddr, localAddr, conn, sessionRoleInbound)
	endpointRef, ensureErr := a.endpointManager.ensureEndpoint(ctx, message.peerAddr, func(address string) (vivid.ActorRef, error) {
		endpointRef, err := ctx.ActorOf(newEndpoint(address, a.endpointManager, a.codec, a.envelopHandler, a.advertiseAddr, a.bufferPolicy, a.retryPolicy, a.associationPolicy))
		if err != nil {
			return nil, err
		}
		return endpointRef, nil
	})
	if ensureErr != nil {
		spawnErr := vivid.ErrorRemotingSessionSpawnFailed.With(ensureErr)
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
		if closeErr := conn.Close(); closeErr != nil {
			return "", errors.Join(err, closeErr)
		}
		return "", err
	}
	peerAddr := request.AdvertiseAddr
	if peerAddr == "" {
		peerAddr = conn.RemoteAddr().String()
	}
	response := &Handshake{AdvertiseAddr: a.advertiseAddr}
	if err := response.Send(conn); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			return "", errors.Join(err, closeErr)
		}
		return "", err
	}
	return peerAddr, nil
}
