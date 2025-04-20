package queues

import (
	"sync/atomic"
	"unsafe"
)

type LFQueue struct {
	head atomic.Pointer[lfNode] // 使用泛型原子指针
	tail atomic.Pointer[lfNode]
}

type lfNode struct {
	value unsafe.Pointer
	next  atomic.Pointer[lfNode] // 原子指针替代unsafe
}

func NewLFQueue() *LFQueue {
	dummy := &lfNode{}
	q := &LFQueue{}
	q.head.Store(dummy)
	q.tail.Store(dummy)
	return q
}

func (q *LFQueue) Push(value unsafe.Pointer) {
	newNode := &lfNode{value: value}

	for {
		// 获取当前tail和它的next指针
		currentTail := q.tail.Load()
		next := currentTail.next.Load()

		// 一致性检查
		if currentTail != q.tail.Load() {
			continue // 其他线程已修改tail，重新开始
		}

		if next == nil {
			// 尝试原子插入新节点
			if currentTail.next.CompareAndSwap(nil, newNode) {
				// 成功插入后尝试推进tail（非必须但帮助后续操作）
				q.tail.CompareAndSwap(currentTail, newNode)
				return
			}
		} else {
			// 帮助推进其他未完成的插入操作
			q.tail.CompareAndSwap(currentTail, next)
		}
	}
}

func (q *LFQueue) Pop() unsafe.Pointer {
	currentHead := q.head.Load()
	next := currentHead.next.Load()

	if next == nil {
		return nil // 空队列
	}

	// 单消费者直接更新head（无需CAS）
	q.head.Store(next)

	// 清理旧头节点数据
	value := next.value
	next.value = nil // 防止内存泄漏
	return value
}
