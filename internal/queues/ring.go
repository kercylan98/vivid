package queues

import (
	"sync"
	"sync/atomic"
	"time"
)

type ringBuffer[T any] struct {
	buffer []T
	head   int64
	tail   int64
	mod    int64
}

type RingQueue[T any] struct {
	len            int64
	content        *ringBuffer[T]
	lock           sync.Mutex
	lastShrinkTime int64
}

func New[T any](initialSize int64) *RingQueue[T] {
	return &RingQueue[T]{
		content: &ringBuffer[T]{
			buffer: make([]T, initialSize),
			head:   0,
			tail:   0,
			mod:    initialSize,
		},
		len: 0,
	}
}

func (q *RingQueue[T]) Push(item T) {
	q.lock.Lock()
	c := q.content
	c.tail = (c.tail + 1) % c.mod
	if c.tail == c.head {
		var fillFactor int64 = 2

		newLen := c.mod * fillFactor
		newBuff := make([]T, newLen)

		for i := int64(0); i < c.mod; i++ {
			buffIndex := (c.tail + i) % c.mod
			newBuff[i] = c.buffer[buffIndex]
		}
		newContent := &ringBuffer[T]{
			buffer: newBuff,
			head:   0,
			tail:   c.mod,
			mod:    newLen,
		}
		q.content = newContent
	}
	atomic.AddInt64(&q.len, 1)
	q.content.buffer[q.content.tail] = item
	q.lock.Unlock()
}

func (q *RingQueue[T]) Length() int64 {
	return atomic.LoadInt64(&q.len)
}

func (q *RingQueue[T]) Empty() bool {
	return q.Length() == 0
}

func (q *RingQueue[T]) Pop() (t T, b bool) {
	if q.Empty() {
		return t, false
	}

	q.lock.Lock()
	c := q.content
	c.head = (c.head + 1) % c.mod
	res := c.buffer[c.head]
	c.buffer[c.head] = t
	atomic.AddInt64(&q.len, -1)
	q.lock.Unlock()
	return res, true
}

func (q *RingQueue[T]) PopMany(count int64) ([]T, bool) {
	if q.Empty() {
		return nil, false
	}

	q.lock.Lock()
	c := q.content

	if count >= q.len {
		count = q.len
	}
	atomic.AddInt64(&q.len, -count)

	buffer := make([]T, count)
	for i := int64(0); i < count; i++ {
		pos := (c.head + 1 + i) % c.mod
		buffer[i] = c.buffer[pos]
		var zero T
		c.buffer[pos] = zero
	}
	c.head = (c.head + count) % c.mod

	q.lock.Unlock()
	return buffer, true
}

// Shrink 尝试缩容队列，返回是否成功缩容
func (q *RingQueue[T]) Shrink() bool {
	q.lock.Lock()
	defer q.lock.Unlock()

	// 检查缩容间隔（30秒）
	now := time.Now().UnixNano()
	lastShrink := atomic.LoadInt64(&q.lastShrinkTime)
	if now-lastShrink < 30*1e9 {
		return false
	}

	c := q.content
	currentLen := atomic.LoadInt64(&q.len)

	// 缩容条件：使用率 < 25% 且容量 > 128
	if currentLen >= c.mod/4 || c.mod <= 128 {
		return false
	}

	// 缩容到原来的 1/2
	newMod := c.mod / 2
	if newMod < 128 {
		newMod = 128
	}

	newBuff := make([]T, newMod)

	// 复制现有元素（从 head+1 到 tail）
	for i := int64(0); i < currentLen; i++ {
		oldPos := (c.head + 1 + i) % c.mod
		newBuff[i] = c.buffer[oldPos]
	}

	q.content = &ringBuffer[T]{
		buffer: newBuff,
		head:   -1,
		tail:   currentLen - 1,
		mod:    newMod,
	}

	atomic.StoreInt64(&q.lastShrinkTime, now)
	return true
}
