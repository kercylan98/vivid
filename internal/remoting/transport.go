package remoting

import (
	"encoding/binary"
	"io"
	"net"
	"sync"
)

// Connection 表示一个网络连接
type Connection interface {
	RemoteAddr() net.Addr
	LocalAddr() net.Addr
	Close() error
}

// Listener 表示网络监听器
type Listener interface {
	Accept() (Connection, error)
	Close() error
	Addr() net.Addr
}

// Transport 定义传输层接口
type Transport interface {
	Dial(address net.Addr) (Connection, error)
	Send(conn Connection, data []byte) error
	Receive(conn Connection) ([]byte, error)
	Close(conn Connection) error
	Listen(address net.Addr) (Listener, error)
}


// tcpConnection TCP连接实现
type tcpConnection struct {
	conn *net.TCPConn
	mu   sync.Mutex
}

func (c *tcpConnection) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *tcpConnection) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *tcpConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// tcpListener TCP监听器实现
type tcpListener struct {
	listener *net.TCPListener
}

func (l *tcpListener) Accept() (Connection, error) {
	conn, err := l.listener.AcceptTCP()
	if err != nil {
		return nil, err
	}
	return &tcpConnection{conn: conn}, nil
}

func (l *tcpListener) Close() error {
	return l.listener.Close()
}

func (l *tcpListener) Addr() net.Addr {
	return l.listener.Addr()
}

// TCPTransport TCP传输实现
type TCPTransport struct{}

func NewTCPTransport() *TCPTransport {
	return &TCPTransport{}
}

func (t *TCPTransport) Dial(address net.Addr) (Connection, error) {
	tcpAddr, ok := address.(*net.TCPAddr)
	if !ok {
		// 尝试转换
		addr, err := net.ResolveTCPAddr("tcp", address.String())
		if err != nil {
			return nil, err
		}
		tcpAddr = addr
	}

	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		return nil, err
	}
	return &tcpConnection{conn: conn}, nil
}

func (t *TCPTransport) Send(conn Connection, data []byte) error {
	tcpConn, ok := conn.(*tcpConnection)
	if !ok {
		return io.ErrClosedPipe
	}

	tcpConn.mu.Lock()
	defer tcpConn.mu.Unlock()

	// 先发送长度（4字节）
	length := uint32(len(data))
	if err := binary.Write(tcpConn.conn, binary.BigEndian, length); err != nil {
		return err
	}

	// 发送数据
	_, err := tcpConn.conn.Write(data)
	return err
}

func (t *TCPTransport) Receive(conn Connection) ([]byte, error) {
	tcpConn, ok := conn.(*tcpConnection)
	if !ok {
		return nil, io.ErrClosedPipe
	}

	tcpConn.mu.Lock()
	defer tcpConn.mu.Unlock()

	// 先读取长度（4字节）
	var length uint32
	if err := binary.Read(tcpConn.conn, binary.BigEndian, &length); err != nil {
		return nil, err
	}

	// 读取数据
	data := make([]byte, length)
	_, err := io.ReadFull(tcpConn.conn, data)
	return data, err
}

func (t *TCPTransport) Close(conn Connection) error {
	return conn.Close()
}

func (t *TCPTransport) Listen(address net.Addr) (Listener, error) {
	tcpAddr, ok := address.(*net.TCPAddr)
	if !ok {
		// 尝试转换
		addr, err := net.ResolveTCPAddr("tcp", address.String())
		if err != nil {
			return nil, err
		}
		tcpAddr = addr
	}

	listener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return nil, err
	}
	return &tcpListener{listener: listener}, nil
}

// GetTransport 获取TCP传输实现
func GetTransport() Transport {
	return NewTCPTransport()
}

