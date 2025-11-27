package future

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/mailbox"
)

var (
	_ vivid.Future[vivid.Message] = (*Future[vivid.Message])(nil)
	_ mailbox.EnvelopAgent        = (*Future[vivid.Message])(nil)
)

func NewFuture[T vivid.Message](timeout time.Duration) *Future[T] {
	future := &Future[T]{
		done: make(chan struct{}),
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
}

func (f *Future[T]) Reply(message vivid.Message) {
	msg, ok := message.(T)
	if !ok {
		f.Close(fmt.Errorf("%w, expected %T, got %T", vivid.ErrFutureMessageTypeMismatch, f.message, message))
		return
	}
	f.message = msg
	close(f.done)
}

func (f *Future[T]) Close(err error) {
	if !f.closed.CompareAndSwap(false, true) {
		return
	}
	f.err = err
	close(f.done)
	if f.timer != nil {
		f.timer.Stop()
	}
}

func (f *Future[T]) Result() (T, error) {
	<-f.done
	return f.message, f.err
}

func (f *Future[T]) Wait() error {
	<-f.done
	return f.err
}
