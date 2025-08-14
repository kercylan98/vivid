package builtinfuture

import (
	"sync/atomic"
	"time"

	"github.com/kercylan98/vivid/pkg/vivid/future"
	"github.com/kercylan98/vivid/pkg/vivid/internal/processor"
)

var (
	_ future.Future[any] = (*Future[any])(nil)
	_ processor.Unit     = (*Future[any])(nil)
)

func New[T any](registry processor.Registry, ref processor.UnitIdentifier, timeout time.Duration) *Future[T] {
	f := &Future[T]{
		registry: registry,
		ref:      ref,
		done:     make(chan struct{}),
		timeout:  timeout,
	}

	// 内部会校验重复，忽略错误
	_ = registry.RegisterUnit(ref, f)

	if timeout > 0 {
		f.timer = time.AfterFunc(f.timeout, func() {
			f.Close(future.ErrorFutureTimeout)
		})
	}

	return f
}

type Future[T any] struct {
	registry processor.Registry
	ref      processor.UnitIdentifier
	done     chan struct{}
	timeout  time.Duration
	timer    *time.Timer
	closed   atomic.Bool
	err      error
	message  T
}

func (f *Future[T]) HandleUserMessage(sender processor.UnitIdentifier, message any) {
	if f.closed.Load() {
		return
	}

	if err, ok := message.(error); ok {
		f.Close(err)
	} else {
		f.message = message.(T)
		f.Close(nil)
	}
}

func (f *Future[T]) HandleSystemMessage(sender processor.UnitIdentifier, message any) {
	f.HandleUserMessage(sender, message)
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
	f.registry.UnregisterUnit(f.ref, f.ref)
}

func (f *Future[T]) Result() (T, error) {
	<-f.done
	return f.message, f.err
}

func (f *Future[T]) Wait() error {
	<-f.done
	return f.err
}
