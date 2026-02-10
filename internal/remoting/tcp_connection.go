package remoting

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"sync"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/remoting/serialize"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
)

func newTCPConnectionActor(client bool, conn net.Conn, advertiseAddr string, codec vivid.Codec, envelopHandler NetworkEnvelopHandler, options ...tcpConnectionActorOption) (*tcpConnectionActor, error) {
	opts := &tcpConnectionActorOptions{}
	for _, option := range options {
		option(opts)
	}

	c := &tcpConnectionActor{
		client:         client,
		conn:           conn,
		advertiseAddr:  advertiseAddr,
		envelopHandler: envelopHandler,
		codec:          codec,
	}
	return c, c.handshake()
}

type tcpConnectionActorOption func(options *tcpConnectionActorOptions)

func withTCPConnectionActorReadFailedHandler(handler vivid.ActorSystemRemotingConnectionReadFailedHandler) tcpConnectionActorOption {
	return func(options *tcpConnectionActorOptions) {
		options.readFailedHandler = handler
	}
}

type tcpConnectionActorOptions struct {
	readFailedHandler vivid.ActorSystemRemotingConnectionReadFailedHandler
}

// tcpConnectionActor TCP连接实现
type tcpConnectionActor struct {
	options        tcpConnectionActorOptions
	conn           net.Conn
	codec          vivid.Codec
	envelopHandler NetworkEnvelopHandler
	advertiseAddr  string
	writeCloseLock sync.RWMutex
	client         bool
	closed         bool
}

func (c *tcpConnectionActor) OnReceive(ctx vivid.ActorContext) {
	switch ctx.Message().(type) {
	case *vivid.OnLaunch:
		c.onLaunch(ctx)
	case net.Conn:
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
}

func (c *tcpConnectionActor) onLaunch(ctx vivid.ActorContext) {
	// 启动 reader 循环
	ctx.TellSelf(c.conn)
}

func (c *tcpConnectionActor) onReadConn(ctx vivid.ActorContext) (fatal bool, err error) {
	// 消息读取
	reader := bufio.NewReader(c.conn)
	lengthBuf := make([]byte, 4)
	if _, err = io.ReadFull(reader, lengthBuf); err != nil {
		// 对等连接已关闭
		if errors.Is(err, io.EOF) {
			return false, nil
		}
		// 当消息读取失败时，意味着连接已断开，终止 Actor
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

	// 消息长度，其中 0 表示连接关闭协议
	msgLen := binary.BigEndian.Uint32(lengthBuf)
	if msgLen == 0 {
		_, _ = c.Write(lengthBuf)
		ctx.Kill(ctx.Ref(), false, "peer closed")
		return false, nil
	}

	// 消息长度超过 4MB 则认为无效
	if msgLen > 4*1024*1024 {
		ctx.Logger().Warn("invalid message length", log.Int64("length", int64(msgLen)))
		ctx.TellSelf(c.conn)
		return false, vivid.ErrorInvalidMessageLength.WithMessage(fmt.Sprintf("length: %d", msgLen))
	}
	msgBuf := make([]byte, msgLen)
	if _, err := io.ReadFull(reader, msgBuf); err != nil {
		// 当消息读取失败时，意味着连接已断开，终止 Actor
		ctx.EventStream().Publish(ctx, ves.RemotingConnectionClosedEvent{
			ConnectionRef: ctx.Ref(),
			RemoteAddr:    c.conn.RemoteAddr().String(),
			LocalAddr:     c.conn.LocalAddr().String(),
			AdvertiseAddr: c.advertiseAddr,
			IsClient:      c.client,
			Reason:        fmt.Sprintf("read message body failed: %v", err),
		})
		ctx.Kill(ctx.Ref(), false, err.Error())
		return true, vivid.ErrorReadMessageBufferFailed.With(err)
	}

	if system,
		agentAddr, agentPath,
		senderAddr, senderPath,
		receiverAddr, receiverPath,
		messageInstance,
		err := serialize.DecodeEnvelopWithRemoting(c.codec, msgBuf); err != nil {
		err = vivid.ErrorRemotingMessageDecodeFailed.With(err)
		// 发布消息解码失败事件
		ctx.EventStream().Publish(ctx, ves.RemotingMessageDecodeFailedEvent{
			ConnectionRef: ctx.Ref(),
			RemoteAddr:    c.conn.RemoteAddr().String(),
			MessageSize:   int(msgLen),
			Error:         err,
		})
		// 消息解码不影响连接的正常使用，继续监听连接
		ctx.TellSelf(c.conn)
		ctx.Logger().Warn("decode remote message failed",
			log.String("sender", ctx.Ref().String()),
			log.String("receiver", ctx.Ref().GetPath()),
			log.Any("err", err),
		)
		return false, nil
	} else {
		// 发布消息接收成功事件
		messageType := "unknown"
		if messageInstance != nil {
			messageType = reflect.TypeOf(messageInstance).String()
		}
		ctx.EventStream().Publish(ctx, ves.RemotingMessageReceivedEvent{
			ConnectionRef: ctx.Ref(),
			RemoteAddr:    c.conn.RemoteAddr().String(),
			MessageType:   messageType,
			MessageSize:   int(msgLen),
			ReceiverPath:  receiverPath,
		})
		// 即便是远程消息处理失败，也继续监听连接
		err = c.envelopHandler.HandleRemotingEnvelop(system, agentAddr, agentPath, senderAddr, senderPath, receiverAddr, receiverPath, messageInstance)
		ctx.TellSelf(c.conn)
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

	// 暂且以长度 0 的数据包作为关闭连接消息，写入成功后不再处理关闭，等待 ACK 关闭
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, 0)
	if _, err := c.conn.Write(data); err != nil {
		// 写入失败时强行关闭
		return c.conn.Close()
	}
	// 设置读超时
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
