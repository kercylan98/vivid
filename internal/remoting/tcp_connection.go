package remoting

import (
	"bufio"
	"encoding/binary"
	"io"
	"net"
	"sync"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/remoting/serialize"
	"github.com/kercylan98/vivid/pkg/log"
)

func newTCPConnectionActor(client bool, conn net.Conn, advertiseAddr string, codec vivid.Codec, envelopHandler NetworkEnvelopHandler) *tcpConnectionActor {
	return &tcpConnectionActor{
		client:         client,
		conn:           conn,
		advertiseAddr:  advertiseAddr,
		envelopHandler: envelopHandler,
		codec:          codec,
	}
}

// tcpConnectionActor TCP连接实现
type tcpConnectionActor struct {
	client         bool
	advertiseAddr  string
	envelopHandler NetworkEnvelopHandler
	writeCloseLock sync.RWMutex
	conn           net.Conn
	closed         bool
	codec          vivid.Codec
}

func (c *tcpConnectionActor) OnPrelaunch() (err error) {
	handshakeProtocol := &Handshake{
		AdvertiseAddr: c.advertiseAddr,
	}

	if c.client {
		if err = handshakeProtocol.Send(c.conn); err != nil {
			return err
		}
		if err = handshakeProtocol.Wait(c.conn); err != nil {
			return err
		}
	} else {
		if err = handshakeProtocol.Wait(c.conn); err != nil {
			return err
		}
		if err = handshakeProtocol.Send(c.conn); err != nil {
			return err
		}
	}

	// 将 Reader 纳入 ServerActor 统一管理，Writer 作为单独同步属性维护
	return nil
}

func (c *tcpConnectionActor) OnReceive(ctx vivid.ActorContext) {
	switch ctx.Message().(type) {
	case *vivid.OnLaunch:
		c.onLaunch(ctx)
	case net.Conn:
		c.onReadConn(ctx)
	}
}

func (c *tcpConnectionActor) onLaunch(ctx vivid.ActorContext) {
	// 启动 reader 循环
	ctx.TellSelf(c.conn)
}

func (c *tcpConnectionActor) onReadConn(ctx vivid.ActorContext) {
	reader := bufio.NewReader(c.conn)
	lengthBuf := make([]byte, 4)
	if _, err := io.ReadFull(reader, lengthBuf); err != nil {
		// 当消息读取失败时，意味着连接已断开，终止 Actor
		ctx.Kill(ctx.Ref(), false, err.Error())
		return
	}
	msgLen := binary.BigEndian.Uint32(lengthBuf)
	if msgLen == 0 || msgLen > 4*1024*1024 {
		ctx.Logger().Warn("invalid message length", log.Int64("length", int64(msgLen)))
		ctx.TellSelf(c.conn)
		return
	}
	msgBuf := make([]byte, msgLen)
	if _, err := io.ReadFull(reader, msgBuf); err != nil {
		// 当消息读取失败时，意味着连接已断开，终止 Actor
		ctx.Kill(ctx.Ref(), false, err.Error())
		return
	}

	if system,
		agentAddr, agentPath,
		senderAddr, senderPath,
		receiverAddr, receiverPath,
		messageInstance,
		err := serialize.DecodeEnvelopWithRemoting(c.codec, msgBuf); err != nil {
		ctx.Logger().Warn("decode Remoting envelop failed", log.Any("err", err))
	} else {
		c.envelopHandler.HandleRemotingEnvelop(system, agentAddr, agentPath, senderAddr, senderPath, receiverAddr, receiverPath, messageInstance)
	}

	ctx.TellSelf(c.conn)
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
	return c.conn.Close()
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
