package server

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"

	"github.com/kercylan98/vivid/pkg/runtime/builtin/registry"
)

var _ registry.Connection = (*tcpConnection)(nil)

// newTCPConnection 创建一个新的 TCP 连接包装器。
func newTCPConnection(conn net.Conn) *tcpConnection {
	return &tcpConnection{
		conn: conn,
	}
}

// tcpConnection 实现了 runtime.Connection 接口，基于 net.Conn。
// 使用长度前缀（4字节大端序）来处理 TCP 粘包问题。
type tcpConnection struct {
	conn net.Conn
}

// Send 实现 Connection 接口。
// 数据格式: | length(uint32) | data(bytes) |
func (c *tcpConnection) Send(data []byte) error {
	if len(data) > 0xFFFFFFFF {
		return fmt.Errorf("data too large: %d bytes", len(data))
	}

	// 写入长度前缀
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, uint32(len(data)))

	if _, err := c.conn.Write(lengthBuf); err != nil {
		return err
	}

	// 写入数据
	if _, err := c.conn.Write(data); err != nil {
		return err
	}

	return nil
}

// Recv 实现 Connection 接口。
func (c *tcpConnection) Recv() ([]byte, error) {
	// 读取长度前缀
	lengthBuf := make([]byte, 4)
	if _, err := io.ReadFull(c.conn, lengthBuf); err != nil {
		return nil, err
	}

	length := binary.BigEndian.Uint32(lengthBuf)
	if length == 0 {
		return []byte{}, nil
	}

	// 读取数据
	data := make([]byte, length)
	if _, err := io.ReadFull(c.conn, data); err != nil {
		return nil, err
	}

	return data, nil
}

// Close 实现 Connection 接口。
func (c *tcpConnection) Close() error {
	return c.conn.Close()
}

// RemoteAddress 实现 Connection 接口。
func (c *tcpConnection) RemoteAddress() string {
	return c.conn.RemoteAddr().String()
}

// LocalAddress 实现 Connection 接口。
func (c *tcpConnection) LocalAddress() string {
	return c.conn.LocalAddr().String()
}
