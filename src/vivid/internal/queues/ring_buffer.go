package queues

import (
	"sync"
)

type RingBuffer struct {
	buffer []any
	lock   sync.Mutex
	head   int64
	tail   int64
	size   int64
	count  int64 // 当前元素数量
}

func NewRingBuffer(initialSize int64) *RingBuffer {
	return &RingBuffer{
		buffer: make([]any, initialSize),
		size:   initialSize,
		head:   0,
		tail:   0,
		count:  0,
	}
}

func (r *RingBuffer) Push(item any) {
	r.lock.Lock()
	defer r.lock.Unlock()

	// 检查是否需要扩容
	if r.count == r.size {
		r.resize()
	}

	r.buffer[r.tail] = item
	r.tail = (r.tail + 1) % r.size
	r.count++
}

func (r *RingBuffer) Pop() any {
	r.lock.Lock()
	defer r.lock.Unlock()

	if r.count == 0 {
		return nil
	}

	item := r.buffer[r.head]
	r.buffer[r.head] = nil // 清理引用，避免内存泄漏
	r.head = (r.head + 1) % r.size
	r.count--

	return item
}

// resize 扩容方法
func (r *RingBuffer) resize() {
	newSize := r.size * 2
	newBuffer := make([]any, newSize)

	// 将环形缓冲区的数据复制到新的线性缓冲区
	for i := int64(0); i < r.count; i++ {
		newBuffer[i] = r.buffer[(r.head+i)%r.size]
	}

	r.buffer = newBuffer
	r.head = 0
	r.tail = r.count
	r.size = newSize
}

// Len 返回当前缓冲区中的元素数量
func (r *RingBuffer) Len() int64 {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.count
}

// Cap 返回当前缓冲区的容量
func (r *RingBuffer) Cap() int64 {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.size
}
