package actor

import (
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/require"
)

func TestScheduler_ManyActors(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { require.NoError(t, system.Stop(5*time.Second)) })
	const count = 512
	completed := make(chan vivid.ScheduledTask, count)
	for index := range count {
		var task vivid.ScheduledTask
		received := 0
		_, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
			switch message := ctx.Message().(type) {
			case *vivid.OnLaunch:
				var err error
				task, err = ctx.Scheduler().Loop(ctx.Ref(), time.Millisecond, index, vivid.WithSchedulerReference("tick"))
				if err != nil {
					t.Error(err)
					completed <- nil
				}
			case int:
				if message != index || !ctx.Ref().Equals(ctx.Sender()) {
					t.Error("大量 Actor 循环任务必须保留消息与发送方")
				}
				received++
				if received == 3 {
					if err := task.Cancel(); err != nil {
						t.Error(err)
					}
					completed <- task
				}
			}
		}))
		require.NoError(t, err)
	}
	for range count {
		task := receiveRetryResult(t, completed)
		require.NotNil(t, task)
		requireScheduledTaskReleased(t, task)
	}
}

func TestScheduler_ParentDeathNotificationFollowsCleanup(t *testing.T) {
	system := NewSystem(vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer func() { unblock(); require.NoError(t, system.Stop(time.Second)) }()
	system.eventStream = &startupEventStream{EventStream: system.eventStream, observe: func(event vivid.Message) {
		if event, ok := event.(ves.ActorKilledEvent); ok && event.ActorRef.GetPath() == "/parent/child" {
			// 固定停在死亡事件发布处，父级的消息处理不能依赖子 Actor 后续步骤。
			<-release
		}
	}}
	require.NoError(t, system.Start())
	childReady := make(chan vivid.ActorRef, 1)
	tasks := make(chan vivid.ScheduledTask, 1)
	observed := make(chan bool, 1)
	_, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch message := ctx.Message().(type) {
		case *vivid.OnLaunch:
			child, err := ctx.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
				if _, ok := ctx.Message().(*vivid.OnKill); ok {
					task, err := ctx.Scheduler().Loop(ctx.Ref(), time.Hour, 1)
					if err != nil {
						t.Error(err)
					}
					tasks <- task
				}
			}), vivid.WithActorName("child"))
			if err != nil {
				t.Error(err)
			}
			childReady <- child
		case *vivid.OnKilled:
			if !message.Ref.Equals(ctx.Ref()) {
				task := <-tasks
				observed <- task != nil && !task.Exists()
			}
		}
	}), vivid.WithActorName("parent"))
	require.NoError(t, err)
	child := receiveRetryResult(t, childReady)
	system.Kill(child, false, "scheduler cleanup")
	require.True(t, receiveRetryResult(t, observed), "父级收到终止通知时，子 Actor 调度登记必须已关闭")
}

func TestScheduler_StopReleasesRootTasks(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	task, err := system.Scheduler().Loop(system.Ref(), time.Hour, 1)
	require.NoError(t, err)
	require.NoError(t, system.Stop(time.Second))
	requireScheduledTaskReleased(t, task)
}

// BenchmarkScheduler_ResidentTasks 在已有大量等待任务时测量新任务的注册与取消。
// 每个等待任务属于独立 Scheduler，共用测试根上下文，不包含同等数量 Actor 的邮箱与行为成本。
func BenchmarkScheduler_ResidentTasks(b *testing.B) {
	for _, count := range []int{1000, 100000, 1000000} {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			system := NewSystem(vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
			if err := system.Start(); err != nil {
				b.Fatal(err)
			}
			b.Cleanup(func() {
				if err := system.Stop(5 * time.Second); err != nil {
					b.Error(err)
				}
			})
			owners := make([]*Scheduler, count)
			b.Cleanup(func() {
				for _, owner := range owners {
					if owner != nil {
						owner.close()
					}
				}
			})
			option := vivid.WithSchedulerReference("tick")
			runtime.GC()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			for index := range owners {
				owners[index] = newScheduler(system.Context)
				if _, err := owners[index].Loop(system.Ref(), time.Hour, 1, option); err != nil {
					b.Fatal(err)
				}
			}
			runtime.GC()
			runtime.ReadMemStats(&after)
			residentBytes := float64(after.HeapAlloc-before.HeapAlloc) / float64(count)
			owner := newScheduler(system.Context)
			defer owner.close()
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				task, err := owner.Once(system.Ref(), time.Hour, 1, option)
				if err != nil {
					b.Fatal(err)
				}
				if err := task.Cancel(); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(residentBytes, "resident-B/task")
			runtime.KeepAlive(owners)
		})
	}
}
