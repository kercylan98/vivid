package queues

import (
	"github.com/kercylan98/vivid/src/internal/queues"
	"sync"
	"sync/atomic"
	"testing"
	"unsafe"
)

const (
	concurrentProducers = 32   // 并发生产者数量
	burstSize           = 1000 // 每次突发推送数量
)

func BenchmarkMPSCQueue(b *testing.B) {
	b.Run("SingleProducer", func(b *testing.B) {
		q := NewMPSC()
		b.ReportAllocs()
		b.ResetTimer()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				q.Push(unsafe.Pointer(&struct{}{}))
			}
		})
	})

	b.Run("MultiProducer", func(b *testing.B) {
		q := queues.NewMPSC()
		var wg sync.WaitGroup
		perProducer := b.N / concurrentProducers

		b.ReportAllocs()
		b.ResetTimer()

		wg.Add(concurrentProducers)
		for i := 0; i < concurrentProducers; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < perProducer; j++ {
					q.Push(unsafe.Pointer(&struct{}{}))
				}
			}()
		}
		wg.Wait()
	})

	b.Run("PushPop", func(b *testing.B) {
		q := queues.NewMPSC()
		var (
			wg      sync.WaitGroup
			counter uint64
		)

		// 消费者协程
		wg.Add(1)
		go func() {
			defer wg.Done()
			for atomic.LoadUint64(&counter) < uint64(b.N) {
				if v := q.Pop(); v != nil {
					atomic.AddUint64(&counter, 1)
				}
			}
		}()

		b.ReportAllocs()
		b.ResetTimer()

		// 生产者组
		wg.Add(concurrentProducers)
		for i := 0; i < concurrentProducers; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < b.N/concurrentProducers; j++ {
					q.Push(unsafe.Pointer(&struct{}{}))
				}
			}()
		}

		wg.Wait()
		b.StopTimer()
	})

	b.Run("BurstPush", func(b *testing.B) {
		q := queues.NewMPSC()
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			// 单次迭代包含burstSize次推送
			for j := 0; j < burstSize; j++ {
				q.Push(unsafe.Pointer(&struct{}{}))
			}
		}
	})
}

func TestZeroAlloc(t *testing.T) {
	q := queues.NewMPSC()
	allocs := testing.AllocsPerRun(1000, func() {
		q.Push(unsafe.Pointer(&struct{}{}))
	})

	if allocs > 0 {
		t.Fatalf("Expected zero allocation, got %.1f allocs/op", allocs)
	}
}
