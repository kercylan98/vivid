package remoting

import (
	"io"
	"net"
	"sync"
	"time"
)

type sessionRole uint8

const (
	sessionRoleInbound sessionRole = iota + 1
	sessionRoleOutbound
)

func newSession(address, localAddress string, conn net.Conn, role sessionRole) *session {
	s := &session{
		address:      address,
		localAddress: localAddress,
		conn:         conn,
		role:         role,
	}
	return s
}

type session struct {
	address      string
	localAddress string
	conn         net.Conn
	role         sessionRole
	writeMu      sync.Mutex
	closed       bool
	stateMu      sync.RWMutex
}

func (s *session) Write(data []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if s.isClosed() {
		return io.ErrClosedPipe
	}
	conn := s.Conn()
	if conn == nil {
		return io.ErrClosedPipe
	}
	_, err := writeFull(conn, data)
	return err
}

func (s *session) WriteFrame(frame Frame) error {
	return s.Write(frame.Bytes())
}

func (s *session) Close() error {
	s.stateMu.Lock()
	if s.closed {
		s.stateMu.Unlock()
		return nil
	}
	s.closed = true
	conn := s.conn
	s.stateMu.Unlock()
	if conn == nil {
		return nil
	}
	_ = conn.SetDeadline(time.Now())
	return conn.Close()
}

func (s *session) isClosed() bool {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.closed
}

// Conn 返回底层连接，供 reader 读帧使用；关闭由 session.Close 统一负责。
func (s *session) Conn() net.Conn {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.conn
}
