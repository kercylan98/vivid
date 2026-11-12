package remoting

import (
	"context"
	"net"
	"sync"
	"time"
)

// pendingConnection 在异步任务结果交给 Actor 前持有连接，避免迟到结果无人释放。
type pendingConnection struct {
	manager *EndpointManager
	ctx     context.Context
	cancel  context.CancelFunc
	mu      sync.Mutex
	conn    net.Conn
	timer   *time.Timer
	done    bool
}

func (e *EndpointManager) newPendingConnection(timeout time.Duration) *pendingConnection {
	ctx, cancel := context.WithCancel(context.Background())
	p := &pendingConnection{manager: e, ctx: ctx, cancel: cancel}
	e.mu.Lock()
	if e.stopped {
		e.mu.Unlock()
		p.close()
		return p
	}
	e.pending[p] = struct{}{}
	e.mu.Unlock()
	if timeout > 0 {
		p.startTimeout(timeout)
	}
	return p
}

func (p *pendingConnection) startTimeout(timeout time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.done && p.timer == nil {
		p.timer = time.AfterFunc(timeout, p.close)
	}
}

func (p *pendingConnection) attach(conn net.Conn) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.done {
		_ = closeConnection(conn)
		return false
	}
	p.conn = conn
	return true
}

// release 将连接交给会话；与超时和停止互斥，成功后不再由待建连任务关闭。
func (p *pendingConnection) release() bool {
	return p.finish(false)
}

func (p *pendingConnection) close() {
	p.finish(true)
}

func (p *pendingConnection) finish(closeConn bool) bool {
	p.mu.Lock()
	if p.done {
		p.mu.Unlock()
		return false
	}
	p.done = true
	conn := p.conn
	p.conn = nil
	if p.timer != nil {
		p.timer.Stop()
	}
	p.mu.Unlock()
	p.cancel()
	if closeConn && conn != nil {
		_ = closeConnection(conn)
	}
	p.manager.mu.Lock()
	delete(p.manager.pending, p)
	p.manager.mu.Unlock()
	return true
}
