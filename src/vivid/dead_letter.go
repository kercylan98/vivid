package vivid

import (
	"container/ring"
	"sync"
	"sync/atomic"
	"time"
)

// deadLetterHandler 死信处理器的默认实现
type deadLetterHandler struct {
	// 死信队列
	deadLetters   *ring.Ring
	deadLettersMu sync.RWMutex

	// 原子计数器
	deadLetterCount int64
	maxQueueSize    int

	// 自定义处理函数
	customHandler func(DeadLetterMessage)

	// 重试机制
	enableRetry   bool
	maxRetries    int
	retryInterval time.Duration
}

// NewDeadLetterHandler 创建新的死信处理器
func NewDeadLetterHandler(queueSize int) DeadLetterHandler {
	return &deadLetterHandler{
		deadLetters:   ring.New(queueSize),
		maxQueueSize:  queueSize,
		enableRetry:   false,
		maxRetries:    3,
		retryInterval: time.Second * 5,
	}
}

// NewDeadLetterHandlerWithRetry 创建带重试功能的死信处理器
func NewDeadLetterHandlerWithRetry(queueSize int, maxRetries int, retryInterval time.Duration) DeadLetterHandler {
	return &deadLetterHandler{
		deadLetters:   ring.New(queueSize),
		maxQueueSize:  queueSize,
		enableRetry:   true,
		maxRetries:    maxRetries,
		retryInterval: retryInterval,
	}
}

func (d *deadLetterHandler) HandleDeadLetter(deadLetter DeadLetterMessage) {
	atomic.AddInt64(&d.deadLetterCount, 1)

	d.deadLettersMu.Lock()
	defer d.deadLettersMu.Unlock()

	// 添加到环形缓冲区
	d.deadLetters.Value = deadLetter
	d.deadLetters = d.deadLetters.Next()

	// 调用自定义处理器
	if d.customHandler != nil {
		go d.customHandler(deadLetter)
	}

	// 重试逻辑
	if d.enableRetry && deadLetter.Attempts < d.maxRetries {
		go d.retryDeadLetter(deadLetter)
	}
}

func (d *deadLetterHandler) GetDeadLetters() []DeadLetterMessage {
	d.deadLettersMu.RLock()
	defer d.deadLettersMu.RUnlock()

	var result []DeadLetterMessage
	d.deadLetters.Do(func(value interface{}) {
		if deadLetter, ok := value.(DeadLetterMessage); ok && !deadLetter.Timestamp.IsZero() {
			result = append(result, deadLetter)
		}
	})

	return result
}

func (d *deadLetterHandler) GetDeadLetterCount() int64 {
	return atomic.LoadInt64(&d.deadLetterCount)
}

func (d *deadLetterHandler) ClearDeadLetters() {
	d.deadLettersMu.Lock()
	defer d.deadLettersMu.Unlock()

	// 重新创建环形缓冲区
	d.deadLetters = ring.New(d.maxQueueSize)
	atomic.StoreInt64(&d.deadLetterCount, 0)
}

// SetCustomHandler 设置自定义死信处理函数
func (d *deadLetterHandler) SetCustomHandler(handler func(DeadLetterMessage)) {
	d.customHandler = handler
}

func (d *deadLetterHandler) retryDeadLetter(deadLetter DeadLetterMessage) {
	time.Sleep(d.retryInterval)

	// 增加重试次数
	deadLetter.Attempts++
	deadLetter.Timestamp = time.Now()

	// 尝试重新发送消息
	// 这里需要访问Actor系统来重新发送消息
	// 实际实现中需要注入ActorSystem引用

	// 如果重新发送失败，再次处理为死信
	if deadLetter.Attempts >= d.maxRetries {
		d.HandleDeadLetter(deadLetter)
	}
}

// SimpleDeadLetterHandler 简单的死信处理器实现
type SimpleDeadLetterHandler struct {
	*deadLetterHandler
}

// NewSimpleDeadLetterHandler 创建简单死信处理器
func NewSimpleDeadLetterHandler(queueSize int) DeadLetterHandler {
	base := NewDeadLetterHandler(queueSize).(*deadLetterHandler)

	handler := &SimpleDeadLetterHandler{
		deadLetterHandler: base,
	}

	// 设置简单的日志处理器
	base.SetCustomHandler(handler.logDeadLetter)

	return handler
}

func (s *SimpleDeadLetterHandler) logDeadLetter(deadLetter DeadLetterMessage) {
	// 简单的日志记录，实际项目中可以集成日志框架
	// fmt.Printf("[DEAD LETTER] From: %s, To: %s, Reason: %s, Attempts: %d\n",
	//	deadLetter.From.String(), deadLetter.To.String(), deadLetter.Reason, deadLetter.Attempts)
}
