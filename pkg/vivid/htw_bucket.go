package vivid

import (
	"container/list"
)

func newHtwBucket() *htwBucket {
	return &htwBucket{
		timers: list.New(),
	}
}

type htwBucket struct {
	timers *list.List // 定时器任务队列
}

// add 添加定时器任务到桶中，返回其元素指针（Actor 单线程上下文，无需加锁）
func (b *htwBucket) add(task *timerTask) *list.Element {
	return b.timers.PushBack(task)
}

// removeElement 通过元素指针移除任务（Actor 单线程上下文，无需加锁）
func (b *htwBucket) removeElement(e *list.Element) {
	if e != nil {
		b.timers.Remove(e)
	}
}

// flush 清空桶并返回所有任务（Actor 单线程上下文，无需加锁）
func (b *htwBucket) flush() []*timerTask {
	var tasks []*timerTask
	for e := b.timers.Front(); e != nil; e = e.Next() {
		task := e.Value.(*timerTask)
		tasks = append(tasks, task)
	}
	b.timers.Init()
	return tasks
}

// size 返回桶中任务数量（Actor 单线程上下文，无需加锁）
func (b *htwBucket) size() int {
	return b.timers.Len()
}

// isEmpty 检查桶是否为空（Actor 单线程上下文，无需加锁）
func (b *htwBucket) isEmpty() bool {
	return b.size() == 0
}
