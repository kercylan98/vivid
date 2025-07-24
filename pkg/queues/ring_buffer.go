package queues

import (
	"sync"
)

// RingBuffer 实现了线程安全的环形缓冲区队列。
//
// RingBuffer 是一个高效的队列实现，使用环形缓冲区来存储数据。
// 它支持动态扩容，当缓冲区满时会自动扩展容量。
//
// 特性：
//   - 线程安全：使用互斥锁保护并发访问
//   - 动态扩容：容量不足时自动扩展为原来的 2 倍
//   - 内存优化：及时清理不再使用的引用
//   - 高效访问：O(1) 时间复杂度的 Push 和 Pop 操作
type RingBuffer struct {
	buffer      []any      // 存储数据的缓冲区
	lock        sync.Mutex // 保护并发访问的互斥锁
	head        int64      // 队列头部索引
	tail        int64      // 队列尾部索引
	size        int64      // 缓冲区总容量
	count       int64      // 当前元素数量
	avgCount    float64    // 历史均值
	statTimes   int64      // 统计次数
	initialSize int64      // 初始容量
}

// NewRingBuffer 创建一个新的环形缓冲区。
//
// 参数 initialSize 指定初始容量，应该是一个正整数。
// 返回一个初始化完成的 RingBuffer 实例。
//
// 示例：
//
//	buffer := NewRingBuffer(1024)
func NewRingBuffer(initialSize int64) *RingBuffer {
	return &RingBuffer{
		buffer:      make([]any, initialSize),
		size:        initialSize,
		head:        0,
		tail:        0,
		count:       0,
		initialSize: initialSize,
	}
}

// Push 将元素添加到队列尾部。
//
// 如果缓冲区已满，会自动扩容为原来的 2 倍。
// 此操作是线程安全的。
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
	r.shrinkIfNeeded()
}

// Pop 从队列头部移除并返回元素。
//
// 如果队列为空，返回 nil。
// 此操作是线程安全的，会自动清理不再使用的引用以避免内存泄漏。
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

	r.shrinkIfNeeded()
	return item
}

// PopN 从队列头部批量移除并返回指定数量的元素。
//
// 参数 n 指定要弹出的元素数量。
// 如果队列中的元素数量少于 n，则返回所有可用元素。
// 返回一个切片，包含弹出的元素。
// 此操作是线程安全的，会自动清理不再使用的引用以避免内存泄漏。
func (r *RingBuffer) PopN(n int64) []any {
	r.lock.Lock()
	defer r.lock.Unlock()

	// 如果队列为空或 n <= 0，返回空切片
	if r.count == 0 || n <= 0 {
		return []any{}
	}

	// 如果请求的数量大于当前元素数量，调整为实际可用数量
	if n > r.count {
		n = r.count
	}

	// 创建结果切片
	result := make([]any, n)

	// 复制元素并清理原引用
	for i := int64(0); i < n; i++ {
		result[i] = r.buffer[r.head]
		r.buffer[r.head] = nil // 清理引用，避免内存泄漏
		r.head = (r.head + 1) % r.size
	}

	r.count -= n
	r.shrinkIfNeeded()
	return result
}

// resize 扩容方法，将缓冲区容量扩展为原来的 2 倍。
//
// 此方法会创建新的缓冲区并复制现有数据，重新整理头尾指针。
// 调用此方法时必须已经持有锁。
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

// Len 返回当前缓冲区中的元素数量。
//
// 此操作是线程安全的。
func (r *RingBuffer) Len() int64 {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.count
}

// Cap 返回当前缓冲区的容量。
//
// 此操作是线程安全的。
func (r *RingBuffer) Cap() int64 {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.size
}

// 收缩逻辑
func (r *RingBuffer) shrinkIfNeeded() {
	// 更新均值
	r.statTimes++
	r.avgCount += (float64(r.count) - r.avgCount) / float64(r.statTimes)

	// 收缩逻辑
	if r.count == 0 && float64(r.size) > r.avgCount && r.size > r.initialSize {
		newSize := int64(r.avgCount)
		if newSize < r.initialSize {
			newSize = r.initialSize
		}
		newBuffer := make([]any, newSize)
		r.buffer = newBuffer
		r.head = 0
		r.tail = 0
		r.size = newSize
	}
}
