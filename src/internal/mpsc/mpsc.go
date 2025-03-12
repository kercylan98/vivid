package queues

import (
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"
)

const (
	cacheLineSize = 64
	chunkSize     = 1024
	slotStateFree = 0
	slotStateUsed = 1
)

type Slot struct {
	value unsafe.Pointer
	state uint64
	_     [cacheLineSize - 16]byte
}

type Chunk struct {
	slots [chunkSize]Slot
	next  unsafe.Pointer // *Chunk
}

type MPSCQueue struct {
	_         [cacheLineSize]byte
	producer  unsafe.Pointer // *Chunk (atomic)
	_         [cacheLineSize]byte
	consumer  unsafe.Pointer // *Chunk
	chunkPool sync.Pool
}

func NewMPSC() *MPSCQueue {
	q := &MPSCQueue{
		chunkPool: sync.Pool{
			New: func() interface{} { return &Chunk{} },
		},
	}

	// 初始化首个块
	initChunk := q.newChunk()
	atomic.StorePointer(&q.producer, unsafe.Pointer(initChunk))
	atomic.StorePointer(&q.consumer, unsafe.Pointer(initChunk))
	return q
}

func (q *MPSCQueue) newChunk() *Chunk {
	c := q.chunkPool.Get().(*Chunk)
	c.next = nil
	for i := range c.slots {
		c.slots[i].state = slotStateFree
	}
	return c
}

func (q *MPSCQueue) Push(value unsafe.Pointer) {
	retries := 0
	for {
		// 获取当前生产块
		c := (*Chunk)(atomic.LoadPointer(&q.producer))

		// 寻找空闲槽位
		for i := 0; i < chunkSize; i++ {
			if atomic.CompareAndSwapUint64(&c.slots[i].state, slotStateFree, slotStateUsed) {
				c.slots[i].value = value
				return
			}
		}

		// 块已满，尝试分配新块
		newChunk := q.newChunk()
		oldNext := atomic.LoadPointer(&c.next)
		if oldNext == nil {
			if atomic.CompareAndSwapPointer(&c.next, nil, unsafe.Pointer(newChunk)) {
				atomic.CompareAndSwapPointer(&q.producer, unsafe.Pointer(c), unsafe.Pointer(newChunk))
			}
		} else {
			atomic.CompareAndSwapPointer(&q.producer, unsafe.Pointer(c), oldNext)
		}

		// 修正后的退避逻辑
		backoff(uint64(retries % 8))
		retries++
	}
}

func (q *MPSCQueue) Pop() (unsafe.Pointer, bool) {
	// 单消费者无需原子操作
	c := (*Chunk)(atomic.LoadPointer(&q.consumer))

	for {
		for i := 0; i < chunkSize; i++ {
			if c.slots[i].state == slotStateUsed {
				val := c.slots[i].value
				c.slots[i].value = nil
				atomic.StoreUint64(&c.slots[i].state, slotStateFree)
				return val, true
			}
		}

		// 当前块消费完毕
		next := (*Chunk)(c.next)
		if next == nil {
			return nil, false
		}

		// 切换到下一个块并回收旧块
		atomic.StorePointer(&q.consumer, c.next)
		old := c
		c = next
		q.recycleChunk(old)
	}
}

func (q *MPSCQueue) recycleChunk(c *Chunk) {
	for i := range c.slots {
		c.slots[i].value = nil
		c.slots[i].state = slotStateFree
	}
	q.chunkPool.Put(c)
}

func backoff(n uint64) {
	for i := uint64(0); i < 1<<(n%8); i++ {
		runtime.Gosched()
	}
}
