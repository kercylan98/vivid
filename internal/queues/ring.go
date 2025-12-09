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

type RingQueue struct {
	len            int64
	content        *ringBuffer
	lock           sync.Mutex
	lastShrinkTime int64
}

func New(initialSize int64) *RingQueue {
	return &RingQueue{
		content: &ringBuffer{
			buffer: make([]interface{}, initialSize),
			head:   0,
			tail:   0,
			mod:    initialSize,
		},
		len: 0,
	}
}

func (q *RingQueue) Push(item any) {
	q.lock.Lock()
	c := q.content
	c.tail = (c.tail + 1) % c.mod
	if c.tail == c.head {
		var fillFactor int64 = 2

		newLen := c.mod * fillFactor
		newBuff := make([]interface{}, newLen)

		for i := int64(0); i < c.mod; i++ {
			buffIndex := (c.tail + i) % c.mod
			newBuff[i] = c.buffer[buffIndex]
		}
		newContent := &ringBuffer{
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

func (q *RingQueue) Length() int64 {
	return atomic.LoadInt64(&q.len)
}

func (q *RingQueue) Empty() bool {
	return q.Length() == 0
}

func (q *RingQueue) Pop() (any, bool) {
	if q.Empty() {
		return nil, false
	}

	q.lock.Lock()
	c := q.content
	c.head = (c.head + 1) % c.mod
	res := c.buffer[c.head]
	c.buffer[c.head] = nil
	atomic.AddInt64(&q.len, -1)
	q.lock.Unlock()
	return res, true
}

func (q *RingQueue) PopMany(count int64) ([]any, bool) {
	if q.Empty() {
		return nil, false
	}

	q.lock.Lock()
	c := q.content

	if count >= q.len {
		count = q.len
	}
	atomic.AddInt64(&q.len, -count)

	buffer := make([]interface{}, count)
	for i := int64(0); i < count; i++ {
		pos := (c.head + 1 + i) % c.mod
		buffer[i] = c.buffer[pos]
		c.buffer[pos] = nil
	}
	c.head = (c.head + count) % c.mod

	q.lock.Unlock()
	return buffer, true
}
