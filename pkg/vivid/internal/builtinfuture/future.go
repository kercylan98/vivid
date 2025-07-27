package builtinfuture

import (
    "github.com/kercylan98/vivid/pkg/vivid/future"
    "github.com/kercylan98/vivid/pkg/vivid/internal/processor"
    "sync/atomic"
    "time"
)

var (
    _ future.Future  = (*Future)(nil)
    _ processor.Unit = (*Future)(nil)
)

func New(registry processor.Registry, ref processor.UnitIdentifier, timeout time.Duration) *Future {
    f := &Future{
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

type Future struct {
    registry processor.Registry
    ref      processor.UnitIdentifier
    done     chan struct{}
    timeout  time.Duration
    timer    *time.Timer
    closed   atomic.Bool
    err      error
    message  any
}

func (f *Future) HandleUserMessage(sender processor.UnitIdentifier, message any) {
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

func (f *Future) HandleSystemMessage(sender processor.UnitIdentifier, message any) {
    f.HandleUserMessage(sender, message)
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
    f.registry.UnregisterUnit(f.ref, f.ref)
}

func (f *Future) Result() (any, error) {
    <-f.done
    var m any
    switch f.message.(type) {
    case nil:
    default:
        m = f.message
    }
    return m, f.err
}

func (f *Future) Wait() error {
    <-f.done
    return f.err
}
