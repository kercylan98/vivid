package actor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/require"
)

type eventStreamLogger struct {
	log.Logger
	observe func(string)
}

func (l *eventStreamLogger) Debug(message string, _ ...any) {
	if l.observe != nil {
		l.observe(message)
	}
}

func (l *eventStreamLogger) Warn(message string, args ...any) {
	l.Debug(message, args...)
}

type eventStreamContext struct {
	ref      vivid.ActorRef
	logger   log.Logger
	onRef    func()
	onLogger func()
}

func (c *eventStreamContext) Ref() vivid.ActorRef {
	if c.onRef != nil {
		c.onRef()
	}
	return c.ref
}

func (c *eventStreamContext) Logger() log.Logger {
	if c.onLogger != nil {
		c.onLogger()
	}
	return c.logger
}

func TestEventStream_Callbacks(t *testing.T) {
	const scenarioEnv = "VIVID_EVENT_STREAM_CALLBACKS"
	if os.Getenv(scenarioEnv) == "" {
		// 重入回归使用独立进程，避免锁回归挂住整个测试套件。
		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestEventStream_Callbacks$", "-test.timeout=15s")
		command.Env = append(os.Environ(), scenarioEnv+"=1")
		output, err := command.CombinedOutput()
		require.NoError(t, err, "%s", output)
		return
	}

	for _, scenario := range []struct {
		name       string
		prepare    bool
		message    string
		subscribed bool
		apply      func(*eventStream, vivid.EventStreamContext)
	}{
		{"subscribe", false, "event subscribed", true, func(es *eventStream, ctx vivid.EventStreamContext) { es.Subscribe(ctx, "") }},
		{"duplicate", true, "event already subscribed", true, func(es *eventStream, ctx vivid.EventStreamContext) { es.Subscribe(ctx, "") }},
		{"missing", false, "event not subscribed", false, func(es *eventStream, ctx vivid.EventStreamContext) { es.Unsubscribe(ctx, "") }},
		{"unsubscribe", true, "event unsubscribed", false, func(es *eventStream, ctx vivid.EventStreamContext) { es.Unsubscribe(ctx, "") }},
		{"all", true, "unsubscribed all events", false, func(es *eventStream, ctx vivid.EventStreamContext) { es.UnsubscribeAll(ctx) }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			for _, mode := range []string{"reentry", "blocked", "panic", "context"} {
				t.Run(mode, func(t *testing.T) {
					es := newEventStream(nil)
					logger := &eventStreamLogger{}
					ctx := &eventStreamContext{ref: &Ref{path: "/subscriber"}, logger: logger}
					if scenario.prepare {
						es.Subscribe(ctx, "")
					}
					var observed bool
					logger.observe = func(message string) {
						require.Equal(t, scenario.message, message)
						observed = true
						logger.observe = nil
						es.mu.RLock()
						_, forward := es.subscribers[reflect.TypeFor[string]()][ctx.ref.GetPath()]
						_, reverse := es.subscriberTypes[ctx.ref.GetPath()][reflect.TypeFor[string]()]
						es.mu.RUnlock()
						require.Equal(t, scenario.subscribed, forward)
						require.Equal(t, forward, reverse)
						if mode == "panic" {
							panic("event logger panic")
						}
						if mode == "blocked" {
							completed := make(chan struct{})
							go func() {
								es.Publish(ctx, 1)
								es.Subscribe(ctx, true)
								es.Unsubscribe(ctx, true)
								close(completed)
							}()
							receiveRetryResult(t, completed)
							return
						}
						// 回调中的变更必须保留，外层返回后不能再覆盖订阅表。
						es.Publish(ctx, 1)
						es.Subscribe(ctx, true)
						es.Unsubscribe(ctx, "")
						es.UnsubscribeAll(ctx)
						es.Subscribe(ctx, true)
					}
					if mode == "context" {
						ctx.onRef = func() { es.Publish(ctx, 1) }
						ctx.onLogger = func() { es.Publish(ctx, 1) }
					}
					if mode == "panic" {
						require.PanicsWithValue(t, "event logger panic", func() { scenario.apply(es, ctx) })
					} else {
						scenario.apply(es, ctx)
					}
					require.True(t, observed)
					if mode == "reentry" || mode == "context" {
						require.Contains(t, es.subscriberTypes[ctx.ref.GetPath()], reflect.TypeFor[bool]())
						require.NotContains(t, es.subscriberTypes[ctx.ref.GetPath()], reflect.TypeFor[string]())
					}
					es.UnsubscribeAll(ctx)
					require.Empty(t, es.subscriberTypes)
				})
			}
		})
	}
}

func TestEventStream_LoggerPublish(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	t.Cleanup(func() { _ = system.Stop(time.Second) })
	stream := system.eventStream.(*eventStream)
	received := make(chan int, 2)
	ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch message := ctx.Message().(type) {
		case *vivid.OnLaunch:
			logger := &eventStreamLogger{}
			context := &eventStreamContext{ref: ctx.Ref(), logger: logger}
			logger.observe = func(string) { stream.Publish(context, 1) }
			stream.Subscribe(context, 0)
			stream.Subscribe(context, 0)
			logger.observe = nil
		case int:
			received <- message
		case string:
			stream.UnsubscribeAll(ctx)
			stream.Publish(ctx, 2)
			ctx.Reply("unsubscribed")
		}
	}))
	require.NoError(t, err)
	require.Equal(t, 1, receiveRetryResult(t, received))
	require.Equal(t, 1, receiveRetryResult(t, received))
	result, err := system.Ask(ref, "unsubscribe", time.Second).Result()
	require.NoError(t, err)
	require.Equal(t, "unsubscribed", result)
	// 重新订阅后由 Actor 生命周期自动清理。
	stream.Subscribe(&eventStreamContext{ref: ref, logger: &eventStreamLogger{}}, 0)
	require.NoError(t, system.Stop(time.Second))
	require.Empty(t, stream.subscriberTypes)
	require.Empty(t, received)
}

func TestEventStream_ConcurrentSubscriptions(t *testing.T) {
	es := newEventStream(nil)
	var workers sync.WaitGroup
	for i := range 16 {
		workers.Go(func() {
			ctx := &eventStreamContext{ref: &Ref{path: fmt.Sprintf("/subscriber-%d", i)}, logger: &eventStreamLogger{}}
			for range 100 {
				es.Subscribe(ctx, "")
				es.Subscribe(ctx, true)
				es.Publish(ctx, 1)
				es.Unsubscribe(ctx, "")
				es.UnsubscribeAll(ctx)
			}
		})
	}
	workers.Wait()
	require.Empty(t, es.subscriberTypes)
	for _, subscribers := range es.subscribers {
		require.Empty(t, subscribers)
	}
}

func TestEventStream_UnsubscribeQueuedEvent(t *testing.T) {
	for _, all := range []bool{false, true} {
		t.Run(fmt.Sprintf("all=%t", all), func(t *testing.T) {
			system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
			entered, release := make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			t.Cleanup(func() { releaseOnce.Do(func() { close(release) }); _ = system.Stop(time.Second) })
			var count int
			ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
				switch ctx.Message().(type) {
				case *vivid.OnLaunch:
					close(entered)
					<-release
				case int:
					count++
				case string:
					ctx.Reply(count)
				}
			}))
			require.NoError(t, err)
			receiveRetryResult(t, entered)
			ctx := &eventStreamContext{ref: ref, logger: &eventStreamLogger{}}
			stream := system.EventStream()
			stream.Subscribe(ctx, 0)
			stream.Publish(ctx, 1)
			if all {
				stream.UnsubscribeAll(ctx)
			} else {
				stream.Unsubscribe(ctx, 0)
			}
			stream.Publish(ctx, 2)
			releaseOnce.Do(func() { close(release) })
			result, err := system.Ask(ref, "count", time.Second).Result()
			require.NoError(t, err)
			require.Equal(t, 1, result, "取消前入队的事件仍处理，取消后的发布不再投递")
		})
	}
}

func BenchmarkEventStream_SubscribeUnsubscribe(b *testing.B) {
	for _, parallel := range []bool{false, true} {
		b.Run(fmt.Sprintf("parallel=%t", parallel), func(b *testing.B) {
			es := newEventStream(nil)
			logger := log.NewTextLogger(log.WithLevel(log.LevelError))
			b.ReportAllocs()
			if parallel {
				var next sync.Mutex
				var id int
				b.RunParallel(func(pb *testing.PB) {
					next.Lock()
					id++
					ctx := &eventStreamContext{ref: &Ref{path: fmt.Sprintf("/subscriber-%d", id)}, logger: logger}
					next.Unlock()
					for pb.Next() {
						es.Subscribe(ctx, "")
						es.Unsubscribe(ctx, "")
					}
				})
				return
			}
			ctx := &eventStreamContext{ref: &Ref{path: "/subscriber"}, logger: logger}
			b.ResetTimer()
			for b.Loop() {
				es.Subscribe(ctx, "")
				es.Unsubscribe(ctx, "")
			}
		})
	}
}

func BenchmarkEventStream_Publish(b *testing.B) {
	system := NewSystem(vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
	require.NoError(b, system.Start())
	b.Cleanup(func() { require.NoError(b, system.Stop()) })
	var consumed sync.WaitGroup
	ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		if _, ok := ctx.Message().(int); ok {
			consumed.Done()
		}
	}))
	require.NoError(b, err)
	stream := system.EventStream()
	stream.Subscribe(&eventStreamContext{ref: ref, logger: &eventStreamLogger{}}, 0)
	consumed.Add(b.N)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		stream.Publish(system, 1)
	}
	consumed.Wait()
	b.StopTimer()
}
