package scheduler

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func testTimers(shards int) *Timers {
	timers := &Timers{shards: make([]*timerQueue, shards)}
	for index := range timers.shards {
		timers.shards[index] = &timerQueue{wake: make(chan struct{}, 1), stopped: make(chan struct{})}
	}
	return timers
}

func receiveTimer[T any](t *testing.T, result <-chan T) T {
	t.Helper()
	select {
	case value := <-result:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("等待计时回调超时")
		var zero T
		return zero
	}
}

func TestTimers_StartsOnlyUsedShards(t *testing.T) {
	timers := NewTimers()
	defer timers.Close()
	for _, queue := range timers.shards {
		require.False(t, queue.started, "未使用的分片不启动等待 goroutine")
	}
	timer := timers.AfterFunc(time.Hour, func() { t.Error("未到期任务不能执行") })
	started := 0
	for _, queue := range timers.shards {
		queue.mu.Lock()
		if queue.started {
			started++
		}
		queue.mu.Unlock()
	}
	require.Equal(t, 1, started)
	require.True(t, timer.Stop())
}

func TestTimers_ResetAndReentry(t *testing.T) {
	timers := testTimers(1)
	defer timers.Close()
	results := make(chan time.Time, 2)
	timer := timers.AfterFunc(time.Hour, func() {
		results <- time.Now()
		timers.AfterFunc(0, func() { results <- time.Now() })
	})
	start := time.Now()
	require.True(t, timer.Reset(5*time.Millisecond), "修改等待中的最近截止时间必须唤醒队列")
	require.GreaterOrEqual(t, receiveTimer(t, results).Sub(start), 5*time.Millisecond, "不能提前执行")
	receiveTimer(t, results)
	require.False(t, timer.Stop(), "已取出的回调不能撤回")
	require.False(t, timer.Reset(time.Hour), "已取出的计时任务可重新登记")
	require.True(t, timer.Stop())
	require.False(t, timer.Stop())
}

func TestTimers_BoundedCallbacksAndClose(t *testing.T) {
	const shards, count = 4, 128
	timers := testTimers(shards)
	defer timers.Close()
	entered, completed := make(chan struct{}, count), make(chan struct{}, count)
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	registered := make([]*Timer, count)
	for index := range registered {
		registered[index] = timers.AfterFunc(0, func() {
			entered <- struct{}{}
			<-release
			completed <- struct{}{}
		})
	}
	for range shards {
		receiveTimer(t, entered)
	}
	for _, queue := range timers.shards {
		queue.mu.Lock()
		waiting := len(queue.heap)
		queue.mu.Unlock()
		require.Equal(t, count/shards-1, waiting, "每个分片最多取出一个回调，其余继续排队")
	}
	stopped := 0
	for _, timer := range registered {
		if timer.Stop() {
			stopped++
		}
	}
	require.Equal(t, count-shards, stopped)
	timers.Close()
	unblock()
	for range shards {
		receiveTimer(t, completed)
	}
	for _, timer := range registered {
		require.False(t, timer.Reset(0), "关闭后不接纳重新登记")
	}
	require.Empty(t, entered, "关闭不重新执行已取消的排队任务")
	late := timers.AfterFunc(0, func() { entered <- struct{}{} })
	require.False(t, late.Stop())
}

func TestTimers_CloseReleasesWaitingCallbacks(t *testing.T) {
	timers := testTimers(2)
	registered := make([]*Timer, 100)
	for index := range registered {
		registered[index] = timers.AfterFunc(time.Hour, func() { t.Error("已关闭的任务不应执行") })
	}
	timers.Close()
	for _, queue := range timers.shards {
		queue.mu.Lock()
		require.Empty(t, queue.heap)
		require.True(t, queue.closed)
		queue.mu.Unlock()
	}
	for _, timer := range registered {
		require.Nil(t, timer.callback)
		require.Equal(t, -1, timer.index)
	}
}

func TestTimers_IndependentSystems(t *testing.T) {
	first, second := testTimers(1), testTimers(1)
	defer second.Close()
	first.AfterFunc(time.Hour, func() { t.Error("已关闭系统不能继续触发") })
	first.Close()
	result := make(chan bool, 1)
	second.AfterFunc(0, func() { result <- true })
	require.True(t, receiveTimer(t, result), "关闭一个系统不影响其他系统的计时队列")
}

func TestTimers_ConcurrentStopResetAndClose(t *testing.T) {
	timers := testTimers(4)
	defer timers.Close()
	var workers sync.WaitGroup
	start := make(chan struct{})
	progress := make(chan struct{}, 16)
	for range 16 {
		workers.Go(func() {
			<-start
			for round := range 1000 {
				timer := timers.AfterFunc(time.Hour, func() { t.Error("未到期任务不能执行") })
				var operations sync.WaitGroup
				operations.Go(func() { timer.Reset(2 * time.Hour) })
				operations.Go(func() { timer.Stop() })
				operations.Wait()
				timer.Stop()
				if round == 49 {
					progress <- struct{}{}
				}
			}
		})
	}
	workers.Go(func() {
		for range 16 {
			<-progress
		}
		timers.Close()
	})
	close(start)
	workers.Wait()
	for _, queue := range timers.shards {
		queue.mu.Lock()
		require.Empty(t, queue.heap)
		queue.mu.Unlock()
	}
}

func TestTimerQueue_HeapChanges(t *testing.T) {
	queue := &timerQueue{}
	random := rand.New(rand.NewSource(98))
	base := time.Now()
	for range 3000 {
		switch random.Intn(3) {
		case 0:
			timer := &Timer{when: base.Add(time.Duration(random.Intn(1000))), index: len(queue.heap)}
			queue.heap = append(queue.heap, timer)
			queue.up(timer.index)
		case 1:
			if len(queue.heap) > 0 {
				queue.remove(random.Intn(len(queue.heap)))
			}
		case 2:
			if len(queue.heap) > 0 {
				index := random.Intn(len(queue.heap))
				queue.heap[index].when = base.Add(time.Duration(random.Intn(1000)))
				queue.fix(index)
			}
		}
		for index, timer := range queue.heap {
			require.Equal(t, index, timer.index)
			if index > 0 {
				require.False(t, timer.when.Before(queue.heap[(index-1)/2].when), "任意取消和重置后仍按截止时间排序")
			}
		}
		for _, timer := range queue.heap[len(queue.heap):cap(queue.heap)] {
			require.Nil(t, timer, "堆尾不得保留已移除的回调")
		}
	}
}
