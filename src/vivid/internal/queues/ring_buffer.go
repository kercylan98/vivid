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

		// 使用copy替代循环复制
		if r.head < r.tail {
			// 如果数据是连续的，直接复制
			copy(newBuff, r.buffer[r.head+1:r.tail+1])
		} else {
			// 如果数据是分段的，分两次复制
			copy(newBuff, r.buffer[r.head+1:])
			copy(newBuff[r.size-r.head-1:], r.buffer[:r.tail+1])
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
