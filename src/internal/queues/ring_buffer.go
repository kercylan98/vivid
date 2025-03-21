package queues

import (
	"sync"
	"sync/atomic"
)

type RingBuffer struct {
	buffer []any
	lock   sync.Mutex
	head   int64
	tail   int64
	size   int64
	len    int64
}

func NewRingBuffer(initialSize int64) *RingBuffer {
	return &RingBuffer{
		buffer: make([]any, initialSize),
		size:   initialSize,
	}
}

func (r *RingBuffer) Push(item any) {
	r.lock.Lock()

	r.tail = (r.tail + 1) % r.size
	if r.tail == r.head {
		newLen := r.size * 2
		newBuff := make([]any, newLen)

		for i := int64(0); i < r.size; i++ {
			buffIndex := (r.tail + i) % r.size
			newBuff[i] = r.buffer[buffIndex]
		}

		r.buffer = newBuff
		r.head = 0
		r.tail = r.size
		r.size = newLen
	}
	atomic.AddInt64(&r.len, 1)
	r.buffer[r.tail] = item

	r.lock.Unlock()
}

func (r *RingBuffer) Length() int64 {
	return atomic.LoadInt64(&r.len)
}

func (r *RingBuffer) Pop() any {
	if atomic.LoadInt64(&r.len) == 0 {
		return nil
	}

	r.lock.Lock()
	r.head = (r.head + 1) % r.size
	res := r.buffer[r.head]
	r.buffer[r.head] = nil
	atomic.AddInt64(&r.len, -1)
	r.lock.Unlock()
	return res
}
