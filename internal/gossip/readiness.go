package gossip

import (
	"sync"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/future"
)

// Readiness 允许系统在 Actor 消息处理之外等待初始加入结果；完成后结果不再改变。
type Readiness struct {
	once   sync.Once
	done   chan struct{}
	result vivid.ClusterJoinResult
	err    error
}

func newReadiness() *Readiness { return &Readiness{done: make(chan struct{})} }

func (r *Readiness) complete(result vivid.ClusterJoinResult, err error) {
	r.once.Do(func() {
		r.result, r.err = result, err
		close(r.done)
	})
}

// Stop 结束尚未完成的等待；系统停止与加入结果之间采用首次完成的结果。
func (r *Readiness) Stop() { r.complete(vivid.ClusterJoinResult{}, vivid.ErrorActorSystemStopped) }

// Wait 为每个调用创建独立 Future，关闭该 Future 不影响其他调用或节点加入。
func (r *Readiness) Wait(liaison vivid.ActorLiaison, timeout time.Duration) vivid.Future[vivid.ClusterJoinResult] {
	waitDone := make(chan struct{})
	f := future.NewFuture[vivid.ClusterJoinResult](liaison, timeout, func() { close(waitDone) })
	finish := func() {
		if r.err != nil {
			f.Close(r.err)
		} else {
			f.EnqueueMessage(r.result)
		}
	}
	select {
	case <-r.done:
		finish()
	default:
		// 等待期间不保留 Actor 消息，也不轮询成员视图；取消后立即退出。
		go func() {
			select {
			case <-r.done:
				finish()
			case <-waitDone:
			}
		}()
	}
	return f
}
