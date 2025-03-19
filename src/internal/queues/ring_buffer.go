package queues

import (
	"sync"
	"sync/atomic"
)

type ringBuffer struct {
	buffer []interface{}
	head   int64
	tail   int64
	mod    int64
}

type RingBuffer struct {
	len     int64
	content *ringBuffer
	lock    sync.Mutex
}

func NewRingBuffer(initialSize int64) *RingBuffer {
	return &RingBuffer{
		content: &ringBuffer{
			buffer: make([]interface{}, initialSize),
			head:   0,
			tail:   0,
			mod:    initialSize,
		},
		len: 0,
	}
}

func (r *RingBuffer) Push(item interface{}) {
	r.lock.Lock()
	c := r.content
	c.tail = (c.tail + 1) % c.mod
	if c.tail == c.head {
		var fillFactor int64 = 2
		// we need to resize

		newLen := c.mod * fillFactor
		newBuff := make([]interface{}, newLen)

		for i := int64(0); i < c.mod; i++ {
			buffIndex := (c.tail + i) % c.mod
			newBuff[i] = c.buffer[buffIndex]
		}
		// set the new buffer and reset head and tail
		newContent := &ringBuffer{
			buffer: newBuff,
			head:   0,
			tail:   c.mod,
			mod:    newLen,
		}
		r.content = newContent
	}
	atomic.AddInt64(&r.len, 1)
	r.content.buffer[r.content.tail] = item
	r.lock.Unlock()
}

func (r *RingBuffer) Length() int64 {
	return atomic.LoadInt64(&r.len)
}

func (r *RingBuffer) Empty() bool {
	return r.Length() == 0
}

func (r *RingBuffer) Pop() (interface{}, bool) {
	if r.Empty() {
		return nil, false
	}
	// as we are a single consumer, no other thread can have poped the items there are guaranteed to be items now

	r.lock.Lock()
	c := r.content
	c.head = (c.head + 1) % c.mod
	res := c.buffer[c.head]
	c.buffer[c.head] = nil
	atomic.AddInt64(&r.len, -1)
	r.lock.Unlock()
	return res, true
}

func (r *RingBuffer) PopMany(count int64) ([]interface{}, bool) {
	if r.Empty() {
		return nil, false
	}

	r.lock.Lock()
	c := r.content

	if count >= r.len {
		count = r.len
	}
	atomic.AddInt64(&r.len, -count)

	buffer := make([]interface{}, count)
	for i := int64(0); i < count; i++ {
		pos := (c.head + 1 + i) % c.mod
		buffer[i] = c.buffer[pos]
		c.buffer[pos] = nil
	}
	c.head = (c.head + count) % c.mod

	r.lock.Unlock()
	return buffer, true
}
