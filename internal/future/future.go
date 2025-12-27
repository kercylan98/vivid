package future

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/kercylan98/vivid"
)

var (
	_ vivid.Future[vivid.Message] = (*Future[vivid.Message])(nil)
	_ vivid.Mailbox               = (*Future[vivid.Message])(nil)
)

func NewFuture[T vivid.Message](timeout time.Duration, closer func()) *Future[T] {
	future := &Future[T]{
		done:   make(chan struct{}),
		closer: closer,
	}

	future.timer = time.AfterFunc(timeout, func() {
		future.Close(vivid.ErrFutureTimeout)
	})

	return future
}

type Future[T vivid.Message] struct {
	done    chan struct{} // 用于通知 future 完成
	timer   *time.Timer   // 超时定时器
	closed  atomic.Bool   // 是否已关闭
	err     error         // 完成时的错误
	message T             // 完成时的消息
	closer  func()        // Future 关闭时的回调函数
}

func (f *Future[T]) Pause() {
	// Future 不考虑暂停
}

func (f *Future[T]) Resume() {
	// Future 不考虑恢复
}

func (f *Future[T]) IsPaused() bool {
	return false
}

// Enqueue 实现 Mailbox 的入列接口，用于 Future 接收消息响应
func (f *Future[T]) Enqueue(envelop vivid.Envelop) {
	f.close(envelop.Message())
}

func (f *Future[T]) Close(err error) {
	f.close(err)
}

func (f *Future[T]) close(v any) {
	if !f.closed.CompareAndSwap(false, true) {
		return
	}
	switch val := v.(type) {
	case error:
		f.err = val
	case T:
		f.message = val
	case nil:
	default:
		f.err = fmt.Errorf("%w, expected %T, got %T", vivid.ErrFutureMessageTypeMismatch, f.message, val)
	}
	close(f.done)
	if f.timer != nil {
		f.timer.Stop()
	}
	if f.closer != nil {
		f.closer()
	}
}

func (f *Future[T]) Result() (T, error) {
	<-f.done
	return f.message, f.err
}

func (f *Future[T]) Wait() error {
	v := <-f.done
	_ = v
	return f.err
}
