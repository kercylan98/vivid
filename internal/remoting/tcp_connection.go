package remoting

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"sync"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/serialization"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
)

func newTCPConnectionActor(client bool, conn net.Conn, advertiseAddr string, codec *serialization.VividCodec, envelopHandler NetworkEnvelopHandler, options ...tcpConnectionActorOption) (*tcpConnectionActor, error) {
	opts := &tcpConnectionActorOptions{}
	for _, option := range options {
		option(opts)
	}
	readTimeout := opts.readTimeout
	if readTimeout <= 0 {
		readTimeout = 30 * time.Second
	}
	c := &tcpConnectionActor{
		client:            client,
		conn:              conn,
		advertiseAddr:     advertiseAddr,
		envelopHandler:    envelopHandler,
		codec:             codec,
		options:           *opts,
		readTimeout:       readTimeout,
		heartbeatInterval: opts.heartbeatInterval,
		stopHeartbeat:     make(chan struct{}),
		headerBuf:         make([]byte, frameHeaderSize),
	}
	return c, c.handshake()
}

type tcpConnectionActorOption func(options *tcpConnectionActorOptions)

func withTCPConnectionActorReadFailedHandler(handler vivid.ActorSystemRemotingConnectionReadFailedHandler) tcpConnectionActorOption {
	return func(options *tcpConnectionActorOptions) {
		options.readFailedHandler = handler
	}
}

func withTCPConnectionActorReadTimeout(d time.Duration) tcpConnectionActorOption {
	return func(options *tcpConnectionActorOptions) {
		options.readTimeout = d
	}
}

func withTCPConnectionActorHeartbeatInterval(d time.Duration) tcpConnectionActorOption {
	return func(options *tcpConnectionActorOptions) {
		options.heartbeatInterval = d
	}
}

type tcpConnectionActorOptions struct {
	readFailedHandler   vivid.ActorSystemRemotingConnectionReadFailedHandler
	readTimeout         time.Duration
	heartbeatInterval   time.Duration
}

// tcpConnectionActor TCP连接实现
type tcpConnectionActor struct {
	options            tcpConnectionActorOptions
	conn               net.Conn
	codec              *serialization.VividCodec
	envelopHandler     NetworkEnvelopHandler
	advertiseAddr      string
	writeCloseLock     sync.RWMutex
	client             bool
	closed             bool
	readTimeout        time.Duration
	heartbeatInterval  time.Duration
	stopHeartbeat      chan struct{}
	headerBuf          []byte
}

func (c *tcpConnectionActor) OnReceive(ctx vivid.ActorContext) {
	switch ctx.Message().(type) {
	case *vivid.OnLaunch:
		c.onLaunch(ctx)
	case net.Conn:
		c.onConn(ctx)
	}
}

func (c *tcpConnectionActor) onLaunch(ctx vivid.ActorContext) {
	if c.heartbeatInterval > 0 {
		go c.runHeartbeat()
	}
	ctx.Tell(ctx.Ref(), c.conn)
}

func (c *tcpConnectionActor) runHeartbeat() {
	ticker := time.NewTicker(c.heartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-c.stopHeartbeat:
			return
		case <-ticker.C:
			if c.Closed() {
				return
			}
			if _, err := c.Write(heartbeatFrameBytes); err != nil {
				return
			}
		}
	}
}

func (c *tcpConnectionActor) onConn(ctx vivid.ActorContext) {
	// 消息读取失败仅作回调，不影响连接的正常使用
	// 假设连接需要关闭，内部会自动关闭连接
	if fatal, err := c.onReadConn(ctx); err != nil && c.options.readFailedHandler != nil {
		// 当非致命错误时，可通过返回 error 来决定是否关闭 Actor
		if err := c.options.readFailedHandler.HandleRemotingConnectionReadFailed(fatal, err); err != nil && !fatal {
			ctx.Kill(ctx.Ref(), false, err.Error())
		}
	} else if err != nil {
		ctx.Logger().Warn("read connection failed",
			log.Bool("fatal", fatal),
			log.Any("err", err),
			log.String("remote_addr", c.conn.RemoteAddr().String()),
			log.String("local_addr", c.conn.LocalAddr().String()),
			log.String("advertise_addr", c.advertiseAddr),
			log.Bool("is_client", c.client))
	}
}

func (c *tcpConnectionActor) onReadConn(ctx vivid.ActorContext) (fatal bool, err error) {
	_ = c.conn.SetReadDeadline(time.Now().Add(c.readTimeout))
	reader := bufio.NewReader(c.conn)
	ctrlType, _, data, err := readFrame(reader, c.headerBuf)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return false, nil
		}
		ctx.EventStream().Publish(ctx, ves.RemotingConnectionClosedEvent{
			ConnectionRef: ctx.Ref(),
			RemoteAddr:    c.conn.RemoteAddr().String(),
			LocalAddr:     c.conn.LocalAddr().String(),
			AdvertiseAddr: c.advertiseAddr,
			IsClient:      c.client,
			Reason:        fmt.Sprintf("read failed: %v", err),
		})
		ctx.Kill(ctx.Ref(), false, err.Error())
		return true, err
	}
	// 每次成功读完整帧后刷新读超时
	_ = c.conn.SetReadDeadline(time.Now().Add(c.readTimeout))

	switch ctrlType {
	case FrameCtrlHandshake:
		// 握手帧应在连接建立时已处理，读循环中收到则忽略并继续
		ctx.Tell(ctx.Ref(), c.conn)
		return false, nil
	case FrameCtrlHeartbeat:
		ctx.Tell(ctx.Ref(), c.conn)
		return false, nil
	case FrameCtrlClose:
		_, _ = c.Write(closeFrameBytes)
		ctx.EventStream().Publish(ctx, ves.RemotingConnectionClosedEvent{
			ConnectionRef: ctx.Ref(),
			RemoteAddr:    c.conn.RemoteAddr().String(),
			LocalAddr:     c.conn.LocalAddr().String(),
			AdvertiseAddr: c.advertiseAddr,
			IsClient:      c.client,
			Reason:        "peer sent close frame",
		})
		ctx.Kill(ctx.Ref(), false, "peer closed")
		return false, nil
	case FrameCtrlData:
		// 常规数据：decodeEnvelop + HandleRemotingEnvelop
		if system, sender, receiver, messageInstance, err := decodeEnvelop(c.codec, data); err != nil {
			err = vivid.ErrorRemotingMessageDecodeFailed.With(err)
			ctx.EventStream().Publish(ctx, ves.RemotingMessageDecodeFailedEvent{
				ConnectionRef: ctx.Ref(),
				RemoteAddr:    c.conn.RemoteAddr().String(),
				MessageSize:   len(data),
				Error:         err,
			})
			ctx.Tell(ctx.Ref(), c.conn)
			ctx.Logger().Warn("decode remote message failed",
				log.String("sender", ctx.Ref().String()),
				log.String("receiver", ctx.Ref().GetPath()),
				log.Any("err", err),
				log.String("bytes", fmt.Sprintf("%s", data)),
			)
			return false, nil
		} else {
			messageType := "unknown"
			if messageInstance != nil {
				messageType = reflect.TypeOf(messageInstance).String()
			}
			ctx.EventStream().Publish(ctx, ves.RemotingMessageReceivedEvent{
				ConnectionRef: ctx.Ref(),
				RemoteAddr:    c.conn.RemoteAddr().String(),
				MessageType:   messageType,
				MessageSize:   len(data),
				Receiver:      receiver,
			})
			err = c.envelopHandler.HandleRemotingEnvelop(system, sender, receiver, messageInstance)
			ctx.Tell(ctx.Ref(), c.conn)
			if err != nil {
				err = vivid.ErrorRemotingMessageHandleFailed.With(err)
				ctx.Logger().Warn("failed to handle remoting message",
					log.String("sender", ctx.Ref().String()),
					log.String("receiver", ctx.Ref().GetPath()),
					log.String("message_type", fmt.Sprintf("%T", messageInstance)),
					log.String("message", fmt.Sprintf("%+v", messageInstance)),
					log.Any("err", err),
				)
			}
			return false, nil
		}
	default:
		ctx.Tell(ctx.Ref(), c.conn)
		return false, nil
	}
}

// Write 暴露给外部的并发安全的写入方法，用于写入消息到连接。
// 参数:
//   - data: 要写入的字节切片
//
// 返回值:
//   - int: 写入的字节数
//   - error: 写入过程中遇到的错误
func (c *tcpConnectionActor) Write(data []byte) (int, error) {
	c.writeCloseLock.Lock()
	defer c.writeCloseLock.Unlock()
	if c.closed {
		return 0, io.EOF
	}
	return c.conn.Write(data)
}

// Close 关闭连接，并返回是否成功。
//
// 返回值:
//   - error: 关闭过程中遇到的错误
func (c *tcpConnectionActor) Close() error {
	c.writeCloseLock.Lock()
	defer c.writeCloseLock.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	close(c.stopHeartbeat)
	if _, err := c.conn.Write(closeFrameBytes); err != nil {
		return c.conn.Close()
	}
	return c.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
}

// Closed 返回连接是否已关闭。
//
// 返回值:
//   - bool: 连接是否已关闭
func (c *tcpConnectionActor) Closed() bool {
	c.writeCloseLock.RLock()
	defer c.writeCloseLock.RUnlock()
	return c.closed
}

func (c *tcpConnectionActor) handshake() (err error) {
	handshakeProtocol := &Handshake{
		AdvertiseAddr: c.advertiseAddr,
	}

	defer func() {
		if err != nil {
			_ = c.conn.Close()
		}
	}()

	if c.client {
		if err = handshakeProtocol.Send(c.conn); err != nil {
			return
		}
		if err = handshakeProtocol.Wait(c.conn); err != nil {
			return
		}
	} else {
		if err = handshakeProtocol.Wait(c.conn); err != nil {
			return
		}
		if err = handshakeProtocol.Send(c.conn); err != nil {
			return
		}
	}

	return nil
}
