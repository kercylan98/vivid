package future

import (
	"fmt"
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
	"github.com/kercylan98/vivid/src/vivid/internal/core/future"
	"github.com/kercylan98/wasteland/src/wasteland"
	"sync/atomic"
	"time"
)

var (
	_                  future.Future            = (*Future)(nil)
	_                  wasteland.ProcessHandler = (*Future)(nil)
	ErrorFutureTimeout                          = fmt.Errorf("future timeout")
)

func New(registry wasteland.ProcessRegistry, ref actor.Ref, timeout time.Duration) *Future {
	f := &Future{
		registry: registry,
		ref:      ref,
		done:     make(chan struct{}),
		timeout:  timeout,
	}

	// 内部会校验重复，忽略错误
	_ = registry.Register(f)

	if timeout > 0 {
		f.timer = time.AfterFunc(f.timeout, func() {
			f.Close(ErrorFutureTimeout)
		})
	}

	return f
}

type Future struct {
	registry wasteland.ProcessRegistry
	ref      actor.Ref
	done     chan struct{}
	timeout  time.Duration
	timer    *time.Timer
	closed   atomic.Bool
	err      error
	message  wasteland.Message
}

func (f *Future) GetID() wasteland.ResourceLocator {
	return f.ref
}

func (f *Future) Close(err error) {
	if !f.closed.CompareAndSwap(false, true) {
		return
	}
	f.err = err
	close(f.done)
	if f.timer != nil {
		f.timer.Stop()
	}
	f.registry.Unregister(f.ref, f.ref)
}

func (f *Future) Result() (any, error) {
	<-f.done
	var m any
	switch f.message.(type) {
	case nil:
	default:
		m = f.message.(any)
	}
	return m, f.err
}

func (f *Future) HandleMessage(sender wasteland.ResourceLocator, priority wasteland.MessagePriority, message wasteland.Message) {
	if f.closed.Load() {
		return
	}

	if err, ok := message.(error); ok {
		f.Close(err)
	} else {
		f.message = message
		f.Close(nil)
	}
}
