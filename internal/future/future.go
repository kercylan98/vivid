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

// NewFuture 创建并返回一个新的 Future，用于异步任务的结果处理与同步控制。
//
// 参数：
//   - timeout：指定 Future 的超时时间。如果该值大于零，到达指定持续时间后 Future 会被自动关闭，并返回 vivid.ErrorFutureTimeout。
//     如果设为零或负数，则不会自动超时关闭。
//   - closer：可选的回调函数，在 Future 被关闭（无论是正常关闭还是超时关闭）时调用，通常用于资源清理或通知用途。如果不需要可传 nil。
//
// 行为说明：
//   - 当 timeout 大于零时，Future 会在超时后自动调用 Close 并传递超时错误（vivid.ErrorFutureTimeout）。
//   - 若传递了 closer，则 Future 关闭时会自动执行该回调。
//   - 返回的 Future 实例可用于后续设置结果或错误，也可等待其完成。
//
// 示例：
//
//	future := NewFuture[MyMessageType](5*time.Second, func() { /* 清理逻辑 */ })
func NewFuture[T vivid.Message](timeout time.Duration, closer func()) *Future[T] {
	future := &Future[T]{
		done:   make(chan struct{}),
		closer: closer,
	}

	if timeout > 0 {
		future.timer = time.AfterFunc(timeout, func() {
			future.Close(vivid.ErrorFutureTimeout)
		})
	}

	return future
}

// NewFutureFail 直接创建失败状态的 Future，用于快速返回错误结果。
func NewFutureFail[T vivid.Message](err error) *Future[T] {
	future := &Future[T]{
		done: make(chan struct{}),
	}
	future.Close(err)
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

func (f *Future[T]) EnqueueMessage(message T) {
	f.close(message)
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
		f.err = fmt.Errorf("%w, expected %T, got %T", vivid.ErrorFutureMessageTypeMismatch, f.message, val)
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
	<-f.done
	return f.err
}
