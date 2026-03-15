package remoting

import (
	"crypto/tls"
	"sync"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/serialization"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
)

var (
	_ vivid.Actor          = (*Remoting)(nil)
	_ vivid.PrelaunchActor = (*Remoting)(nil)
)

func New(bindAddr, advertiseAddr string, codec *serialization.VividCodec, envelopHandler NetworkEnvelopHandler, options vivid.ActorSystemRemotingOptions) *Remoting {
	retryPolicy := newEndpointRetryPolicy(&options)
	bufferPolicy := newEndpointBufferPolicy(&options)
	associationPolicy := newEndpointAssociationPolicy(&options)
	stopTimeout := options.StopTimeout
	if stopTimeout <= 0 {
		stopTimeout = time.Minute
	}
	return &Remoting{
		bindAddr:           bindAddr,
		tlsConfig:          options.TLSConfig,
		codec:              codec,
		envelopHandler:     envelopHandler,
		advertiseAddr:      advertiseAddr,
		bufferPolicy:       bufferPolicy,
		retryPolicy:        retryPolicy,
		associationPolicy:  associationPolicy,
		stopTimeout:        stopTimeout,
		phaseKillCompleted: make(chan struct{}),
	}
}

type Remoting struct {
	bindAddr           string
	tlsConfig          *tls.Config
	codec              *serialization.VividCodec
	envelopHandler     NetworkEnvelopHandler
	advertiseAddr      string
	bufferPolicy       endpointBufferPolicy
	retryPolicy        endpointRetryPolicy
	associationPolicy  endpointAssociationPolicy
	stopTimeout        time.Duration
	listener           *listener
	stopping           bool
	phaseKillCompleted chan struct{}
	phaseKillOnce      sync.Once
	endpointManager    *EndpointManager
}

// OnPrelaunch implements [vivid.PrelaunchActor].
func (r *Remoting) OnPrelaunch(ctx vivid.PrelaunchContext) error {
	r.endpointManager = newEndpointManager()
	// 注册多阶段终止以支持优雅退出
	return ctx.WithPhaseKill(r.phaseKillCompleted, r.stopTimeout, r.OnReceive)
}

func (r *Remoting) OnReceive(ctx vivid.ActorContext) {
	switch message := ctx.Message().(type) {
	case *vivid.OnLaunch:
		r.onLaunch(ctx)
	case vivid.Envelop:
		r.onEnvelop(ctx, message)
	case *vivid.OnKill:
		r.onKill(ctx)
	case *vivid.OnKilled:
		r.onKilled(ctx, message)
	}
}

func (r *Remoting) onLaunch(ctx vivid.ActorContext) {
	listener, err := newListener(r.bindAddr, r.tlsConfig)
	if err != nil {
		ctx.Failed(vivid.ErrorRemotingListen.With(err))
		return
	}

	r.listener = listener
	if _, err = ctx.ActorOf(newAcceptor(listener, r.endpointManager, r.advertiseAddr, r.codec, r.envelopHandler, r.bufferPolicy, r.retryPolicy, r.associationPolicy), vivid.WithActorName("acceptor")); err != nil {
		ctx.Failed(vivid.ParseError(err))
		return
	}
}

func (r *Remoting) onEnvelop(ctx vivid.ActorContext, envelop vivid.Envelop) {
	if r.stopping {
		ctx.EventStream().Publish(ctx, ves.RemotingEnvelopSendFailedEvent{
			TargetAddress: ensureEnvelopReceiverAddress(envelop),
			Error:         vivid.ErrorRemotingStopped,
		})
		r.envelopHandler.HandleFailedRemotingEnvelop(envelop)
		return
	}
	if err := r.endpointManager.EmitToEndpoint(ctx, envelop, func(address string) (vivid.ActorRef, error) {
		endpointRef, err := ctx.ActorOf(newEndpoint(address, r.endpointManager, r.codec, r.envelopHandler, r.advertiseAddr, r.bufferPolicy, r.retryPolicy, r.associationPolicy))
		if err != nil {
			return nil, err
		}
		return endpointRef, nil
	}); err != nil {
		ctx.EventStream().Publish(ctx, ves.RemotingEnvelopSendFailedEvent{
			TargetAddress: ensureEnvelopReceiverAddress(envelop),
			Error:         err,
		})
		ctx.Failed(vivid.ErrorRemotingEndpointSendData.With(err))
	}
}

func (r *Remoting) onKill(ctx vivid.ActorContext) {
	r.stopping = true
	r.endpointManager.Stop()

	// 关闭监听器，停止接受新连接。
	if r.listener != nil {
		if err := r.listener.Close(); err != nil {
			ctx.Logger().Warn("listener close failed", log.Any("error", err))
		}
		r.listener = nil
	}

	for _, child := range ctx.Children() {
		ctx.Kill(child, false, "phase kill")
	}

	if ctx.Children().Len() == 0 {
		r.phaseKillOnce.Do(func() { close(r.phaseKillCompleted) })
	}
}

func (r *Remoting) onKilled(ctx vivid.ActorContext, message *vivid.OnKilled) {
	if !ctx.Ref().Equals(message.Ref) && ctx.Children().Len() == 0 {
		r.phaseKillOnce.Do(func() { close(r.phaseKillCompleted) })
	}
}
