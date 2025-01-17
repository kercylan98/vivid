package vivid

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

const (
	DefaultFutureTimeout = time.Second
)

var ErrorFutureTimeout = fmt.Errorf("future timeout")

var (
	_ Future[Message] = (*future[Message])(nil) // 确保 future 实现了 Future 接口
	_ Process         = (*future[Message])(nil) // 确保 future 实现了 Process 接口
)

// newFuture 创建一个 Future
func newFuture[M Message](sys *actorSystem, ref ActorRef, timeout time.Duration) Future[M] {
	f := &future[M]{
		sys:     sys,
		ref:     ref,
		done:    make(chan struct{}),
		timeout: timeout,
	}

	_, exist, err := sys.processManager.registerProcess(f)
	if err != nil {
		panic(err)
	}

	if exist {
		panic(fmt.Errorf("future actor %s already exists", ref.String()))
	}

	if timeout > 0 {
		f.timer = time.AfterFunc(f.timeout, func() {
			f.Close(ErrorFutureTimeout)
		})
	}

	return f
}

// newFailFuture 创建一个失败的 Future
func newFailFuture[M Message](err error) Future[M] {
	fp := &future[M]{
		done: make(chan struct{}),
	}
	fp.Close(err)
	return fp
}

type Future[M Message] interface {
	// Ref 返回该 Future 的 ActorRef
	Ref() ActorRef

	// Result 阻塞地等待结果
	Result() (M, error)

	// OnlyResult 阻塞地等待结果，不关心错误，如果发送错误将会返回空指针
	OnlyResult() M

	// AssertResult 阻塞地等待结果，当发生错误时将会引发 panic
	AssertResult() M

	// Wait 阻塞的等待结果，该方式不关心结果，仅关心是否成功
	Wait() error

	// AssertWait 阻塞的等待结果，该方式不关心结果，仅关心是否成功，当发生错误时将会引发 panic
	AssertWait()

	// Forward 将结果转发给其他的 ActorRef
	Forward(refs ...ActorRef)

	// Close 提前关闭
	Close(reason error)

	// AwaitForward 异步地等待阻塞结束后向目标 Actor 转发消息
	AwaitForward(ref ActorRef, asyncFunc func() M)
}

type future[M Message] struct {
	sys           *actorSystem
	ref           ActorRef
	timer         *time.Timer
	done          chan struct{}
	message       any
	err           error
	timeout       time.Duration
	forwards      []ActorRef
	closed        atomic.Bool
	forwardsMutex sync.Mutex
}

func (f *future[M]) GetID() ID {
	return f.ref
}

func (f *future[M]) Send(envelope Envelope) {
	if f.closed.Load() {
		return
	}

	message := envelope.GetMessage()

	if err, ok := message.(error); ok {
		f.Close(err)
	} else {
		f.message = message
		f.Close(nil)
	}
}

func (f *future[M]) Terminated() bool {
	return f.closed.Load()
}

func (f *future[M]) OnTerminate(operator ID) {
	// do nothing
}

func (f *future[M]) OnlyResult() (m M) {
	result, err := f.Result()
	if err != nil {
		return m
	}
	return result
}

func (f *future[M]) AwaitForward(ref ActorRef, asyncFunc func() M) {
	f.Forward(ref)
	go func() {
		if reason := recover(); reason != nil {
			process, _ := f.sys.processManager.getProcess(ref)
			process.Send(f.sys.config.FetchRemoteMessageBuilder().BuildStandardEnvelope(f.ref, ref, UserMessage, reason))
		}
		m := asyncFunc()

		process, _ := f.sys.processManager.getProcess(ref)
		process.Send(f.sys.config.FetchRemoteMessageBuilder().BuildStandardEnvelope(f.ref, ref, UserMessage, m))
	}()
}

func (f *future[M]) Ref() ActorRef {
	return f.ref
}

func (f *future[M]) Result() (M, error) {
	<-f.done
	var m M
	switch f.message.(type) {
	case nil:
	default:
		m = f.message.(M)
	}
	return m, f.err
}

func (f *future[M]) AssertResult() M {
	result, err := f.Result()
	if err != nil {
		panic(err)
	}
	return result
}

func (f *future[M]) Wait() error {
	_, err := f.Result()
	return err
}

func (f *future[M]) AssertWait() {
	if err := f.Wait(); err != nil {
		panic(err)
	}
}

func (f *future[M]) Forward(refs ...ActorRef) {
	f.forwardsMutex.Lock()
	defer f.forwardsMutex.Unlock()
	f.forwards = append(f.forwards, refs...)
	if f.closed.Load() {
		f.execForward()
	}
}

func (f *future[M]) Close(reason error) {
	if !f.closed.CompareAndSwap(false, true) {
		return
	}
	f.err = reason
	close(f.done)
	if f.timer != nil {
		f.timer.Stop()
	}
	f.sys.processManager.unregisterProcess(f.ref, f.ref)
	f.forwardsMutex.Lock()
	defer f.forwardsMutex.Unlock()
	f.execForward()
}

func (f *future[M]) execForward() {
	if len(f.forwards) == 0 {
		return
	}

	var m Message
	if f.err != nil {
		m = f.err
	}

	for _, ref := range f.forwards {
		process, _ := f.sys.processManager.getProcess(ref)
		process.Send(f.sys.config.FetchRemoteMessageBuilder().BuildStandardEnvelope(f.ref, ref, UserMessage, m))
	}
	f.forwards = nil
}
