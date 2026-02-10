package remoting

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"reflect"
	"sync"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/remoting/serialize"
	"github.com/kercylan98/vivid/internal/sugar"
	"github.com/kercylan98/vivid/internal/utils"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
	"golang.org/x/sync/singleflight"
)

// eventStreamContext 实现 EventStreamContext 接口，用于在 Mailbox 中发布事件
type eventStreamContext struct {
	ref    vivid.ActorRef
	logger log.Logger
}

func (e *eventStreamContext) Ref() vivid.ActorRef {
	return e.ref
}

func (e *eventStreamContext) Logger() log.Logger {
	return e.logger
}

var (
	_ vivid.Mailbox = &Mailbox{}
)

func newMailbox(ctx context.Context, advertiseAddress string, codec vivid.Codec, envelopHandler NetworkEnvelopHandler, actorLiaison vivid.ActorLiaison, remotingServerRef vivid.ActorRef, eventStream vivid.EventStream, options vivid.ActorSystemRemotingOptions) *Mailbox {
	return &Mailbox{
		ctx:               ctx,
		options:           options,
		advertiseAddress:  advertiseAddress,
		sf:                &singleflight.Group{},
		envelopHandler:    envelopHandler,
		actorLiaison:      actorLiaison,
		remotingServerRef: remotingServerRef,
		codec:             codec,
		eventStream:       eventStream,
		backoff:           utils.NewExponentialBackoffWithDefault(100*time.Millisecond, 3*time.Second),
	}
}

type Mailbox struct {
	ctx               context.Context
	options           vivid.ActorSystemRemotingOptions
	advertiseAddress  string
	connection        *tcpConnectionActor
	connectionLock    sync.RWMutex
	sf                *singleflight.Group
	envelopHandler    NetworkEnvelopHandler
	actorLiaison      vivid.ActorLiaison
	remotingServerRef vivid.ActorRef
	codec             vivid.Codec
	eventStream       vivid.EventStream
	backoff           *utils.ExponentialBackoff
}

func (m *Mailbox) Pause() {
	// 远程邮箱不考虑，该邮箱仅作为向外投递消息的中转通道
}

func (m *Mailbox) Resume() {
	// 远程邮箱不考虑，该邮箱仅作为向外投递消息的中转通道
}

func (m *Mailbox) IsPaused() bool {
	return false
}

func (m *Mailbox) Enqueue(envelop vivid.Envelop) {
	m.connectionLock.Lock()
	defer m.connectionLock.Unlock()

	limit := sugar.Max(m.options.ReconnectLimit, 0)
	_, err := m.backoff.Try(limit, func() (abort bool, err error) {
		if m.ctx.Err() != nil {
			return true, vivid.ErrorActorSystemStopped.With(m.ctx.Err())
		}

		conn, err := m.getOrCreateConnection()
		if err != nil {
			return false, err
		}
		m.connection = conn

		data, err := m.encodeEnvelopWithLength(envelop)
		if err != nil {
			m.onEncodeFailed(envelop, err)
			return true, err
		}

		if m.connection.Closed() {
			m.connection = nil
			return false, fmt.Errorf("connection closed before write")
		}

		if _, err = m.connection.Write(data); err != nil {
			m.connection = nil
			m.publishMessageSendFailed(envelop, vivid.ErrorRemotingMessageSendFailed.With(err))
			return false, err
		}

		m.publishMessageSent(envelop, len(data))
		return true, nil
	})

	if err != nil {
		m.envelopHandler.HandleFailedRemotingEnvelop(envelop)
	}
}

// getOrCreateConnection 在 singleflight 内获取或创建 TCP 连接，调用方需已持 connectionLock。
func (m *Mailbox) getOrCreateConnection() (*tcpConnectionActor, error) {
	v, err, _ := m.sf.Do("init", func() (any, error) {
		if m.connection != nil {
			return m.connection, nil
		}
		conn, err := net.Dial("tcp", m.advertiseAddress)
		if err != nil {
			publishRemotingConnectionFailedEvent(m, m.advertiseAddress, m.advertiseAddress, err, m.backoff.GetAttempt())
			return nil, err
		}
		tcpConn, err := newTCPConnectionActor(true, conn, m.advertiseAddress, m.codec, m.envelopHandler, withTCPConnectionActorReadFailedHandler(m.options.ConnectionReadFailedHandler))
		if err != nil {
			m.actorLiaison.Logger().Warn("handshake failed", log.String("advertise_address", m.advertiseAddress), log.Any("err", err))
			return nil, vivid.ErrorRemotingHandshakeFailed.With(err)
		}
		m.actorLiaison.Logger().Debug("handshake success", log.String("advertise_address", m.advertiseAddress))

		if m.ctx.Err() != nil {
			_ = tcpConn.Close()
			return nil, vivid.ErrorActorSystemStopped.With(m.ctx.Err())
		}
		if err = m.actorLiaison.Ask(m.remotingServerRef, tcpConn).Wait(); err != nil {
			if closeErr := tcpConn.Close(); closeErr != nil {
				return nil, fmt.Errorf("%w, %s", err, closeErr)
			}
			publishRemotingConnectionFailedEvent(m, m.advertiseAddress, m.advertiseAddress, err, m.backoff.GetAttempt())
			return nil, err
		}
		return tcpConn, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*tcpConnectionActor), nil
}

func (m *Mailbox) encodeEnvelopWithLength(envelop vivid.Envelop) ([]byte, error) {
	data, err := serialize.EncodeEnvelopWithRemoting(m.codec, envelop)
	if err != nil {
		return nil, err
	}
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, uint32(len(data)))
	return append(lengthBuf, data...), nil
}

func envelopMessageType(envelop vivid.Envelop) string {
	if envelop.Message() == nil {
		return "unknown"
	}
	return reflect.TypeOf(envelop.Message()).String()
}

func (m *Mailbox) eventContext() (*eventStreamContext, bool) {
	ctx, ok := m.actorLiaison.(vivid.ActorContext)
	if !ok {
		return nil, false
	}
	return &eventStreamContext{ref: m.remotingServerRef, logger: ctx.Logger()}, true
}

func (m *Mailbox) publishMessageSendFailed(envelop vivid.Envelop, err error) {
	if eventCtx, ok := m.eventContext(); ok {
		m.eventStream.Publish(eventCtx, ves.RemotingMessageSendFailedEvent{
			ConnectionRef: nil,
			RemoteAddr:    m.advertiseAddress,
			MessageType:   envelopMessageType(envelop),
			Error:         err,
		})
	}
}

func (m *Mailbox) publishMessageSent(envelop vivid.Envelop, dataLen int) {
	if eventCtx, ok := m.eventContext(); ok {
		m.eventStream.Publish(eventCtx, ves.RemotingMessageSentEvent{
			ConnectionRef: nil,
			RemoteAddr:    m.advertiseAddress,
			MessageType:   envelopMessageType(envelop),
			MessageSize:   dataLen,
		})
	}
}

func (m *Mailbox) onEncodeFailed(envelop vivid.Envelop, err error) {
	err = vivid.ErrorRemotingMessageEncodeFailed.With(err)
	m.publishMessageSendFailed(envelop, err)
	m.actorLiaison.Logger().Warn("failed to enqueue message encode failed",
		log.String("advertise_address", m.advertiseAddress),
		log.String("sender", envelop.Sender().GetPath()),
		log.String("receiver", envelop.Receiver().GetPath()),
		log.String("message_type", fmt.Sprintf("%T", envelop.Message())),
		log.String("message", fmt.Sprintf("%+v", envelop.Message())),
		log.Any("err", err),
	)
}

func publishRemotingConnectionFailedEvent(mailbox *Mailbox, remoteAddr string, advertiseAddr string, error error, retryCount int) {
	eventCtx := &eventStreamContext{
		ref:    mailbox.remotingServerRef,
		logger: mailbox.actorLiaison.Logger(),
	}
	mailbox.eventStream.Publish(eventCtx, ves.RemotingConnectionFailedEvent{
		RemoteAddr:    remoteAddr,
		AdvertiseAddr: advertiseAddr,
		Error:         error,
		RetryCount:    retryCount,
	})

	mailbox.actorLiaison.Logger().Warn("remote connection failed",
		log.String("remote_addr", remoteAddr),
		log.String("advertise_addr", advertiseAddr),
		log.Any("error", error),
		log.Int("retry_count", retryCount),
	)
}
