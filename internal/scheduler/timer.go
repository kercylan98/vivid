package scheduler

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

const timerShardsPerProcessor = 4

// Timers 在系统内分片管理等待任务，每个分片串行执行投递回调。
// 回调只负责向邮箱投递，不执行 Actor 行为；阻塞的投递会延后同分片的其他任务。
type Timers struct {
	shards []*timerQueue
	next   atomic.Uint64
}

func NewTimers() *Timers {
	timers := &Timers{shards: make([]*timerQueue, runtime.GOMAXPROCS(0)*timerShardsPerProcessor)}
	for index := range timers.shards {
		timers.shards[index] = &timerQueue{wake: make(chan struct{}, 1), stopped: make(chan struct{})}
	}
	return timers
}

// AfterFunc 登记一次等待，不为任务单独启动 goroutine 或标准计时器。
func (t *Timers) AfterFunc(delay time.Duration, callback func()) *Timer {
	queue := t.shards[(t.next.Add(1)-1)%uint64(len(t.shards))]
	timer := &Timer{queue: queue, callback: callback, index: -1}
	timer.Reset(delay)
	return timer
}

// Close 停止等待并释放排队回调，不等待已经取出的投递返回。
func (t *Timers) Close() {
	for _, queue := range t.shards {
		queue.mu.Lock()
		if !queue.closed {
			queue.closed = true
			for index, timer := range queue.heap {
				timer.index = -1
				timer.callback = nil
				queue.heap[index] = nil
			}
			queue.heap = nil
			close(queue.stopped)
		}
		queue.mu.Unlock()
	}
}

// Timer 的登记与取出由所属队列锁协调，停止不能撤回已取出的回调。
type Timer struct {
	queue    *timerQueue
	callback func()
	when     time.Time
	index    int // -1 表示已取出或未登记
}

func (t *Timer) Stop() bool {
	queue := t.queue
	queue.mu.Lock()
	defer queue.mu.Unlock()
	if t.index < 0 {
		return false
	}
	first := t.index == 0
	queue.remove(t.index)
	if first {
		queue.notify()
	}
	return true
}

func (t *Timer) Reset(delay time.Duration) bool {
	queue := t.queue
	queue.mu.Lock()
	defer queue.mu.Unlock()
	if queue.closed {
		return false
	}
	active := t.index >= 0
	first := t.index == 0
	t.when = time.Now().Add(max(0, delay))
	if active {
		queue.fix(t.index)
	} else {
		t.index = len(queue.heap)
		queue.heap = append(queue.heap, t)
		queue.up(t.index)
	}
	if first || t.index == 0 {
		queue.notify()
	}
	if !queue.started {
		queue.started = true
		go queue.run()
	}
	return active
}

type timerQueue struct {
	mu      sync.Mutex
	heap    []*Timer
	wake    chan struct{}
	stopped chan struct{}
	started bool
	closed  bool
}

func (q *timerQueue) notify() {
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

func (q *timerQueue) run() {
	wait := time.NewTimer(time.Hour)
	wait.Stop()
	defer wait.Stop()
	for {
		q.mu.Lock()
		if q.closed {
			q.mu.Unlock()
			return
		}
		var deadline <-chan time.Time
		if len(q.heap) > 0 {
			timer := q.heap[0]
			delay := time.Until(timer.when)
			if delay <= 0 {
				q.remove(0)
				callback := timer.callback
				q.mu.Unlock()
				callback()
				continue
			}
			wait.Reset(delay)
			deadline = wait.C
		} else {
			wait.Stop()
		}
		q.mu.Unlock()
		select {
		case <-deadline:
		case <-q.wake:
		case <-q.stopped:
			return
		}
	}
}

func (q *timerQueue) swap(left, right int) {
	q.heap[left], q.heap[right] = q.heap[right], q.heap[left]
	q.heap[left].index, q.heap[right].index = left, right
}

func (q *timerQueue) up(index int) int {
	for index > 0 {
		parent := (index - 1) / 2
		if !q.heap[index].when.Before(q.heap[parent].when) {
			break
		}
		q.swap(index, parent)
		index = parent
	}
	return index
}

func (q *timerQueue) fix(index int) {
	if q.up(index) != index {
		return
	}
	for {
		left := index*2 + 1
		if left >= len(q.heap) {
			return
		}
		child := left
		if right := left + 1; right < len(q.heap) && q.heap[right].when.Before(q.heap[left].when) {
			child = right
		}
		if !q.heap[child].when.Before(q.heap[index].when) {
			return
		}
		q.swap(index, child)
		index = child
	}
}

func (q *timerQueue) remove(index int) {
	last := len(q.heap) - 1
	timer := q.heap[index]
	if index != last {
		q.swap(index, last)
	}
	q.heap[last] = nil
	q.heap = q.heap[:last]
	timer.index = -1
	if index != last {
		q.fix(index)
	}
}
