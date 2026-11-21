package actor

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/messages"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/require"
)

func requireEventStreamSubscriptions(t *testing.T, stream *eventStream, ref vivid.ActorRef, events ...any) {
	t.Helper()
	stream.mu.RLock()
	defer stream.mu.RUnlock()
	types := make(map[reflect.Type]struct{}, len(events))
	for _, event := range events {
		types[reflect.TypeOf(event)] = struct{}{}
	}
	require.Len(t, stream.subscriberTypes[ref.GetPath()], len(types))
	for eventType, subscribers := range stream.subscribers {
		_, subscribed := subscribers[ref.GetPath()]
		_, expected := types[eventType]
		require.Equal(t, expected, subscribed, "事件类型 %v 的正向登记", eventType)
	}
	for eventType := range types {
		require.Contains(t, stream.subscriberTypes[ref.GetPath()], eventType)
		require.Same(t, ref, stream.subscribers[eventType][ref.GetPath()])
	}
}

func TestEventStream_RestartFailure(t *testing.T) {
	for _, scenario := range []string{"provider", "provider_panic", "restarted", "restarted_panic", "prelaunch", "prelaunch_panic"} {
		for _, stop := range []bool{false, true} {
			name := scenario + "/kill"
			if stop {
				name = scenario + "/stop"
			}
			t.Run(name, func(t *testing.T) {
				logger := log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))
				system := NewTestSystem(t, vivid.WithActorSystemLogger(logger))
				t.Cleanup(func() { _ = system.Stop(time.Second) })
				stream := system.eventStream.(*eventStream)
				var observed int
				observer, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
					switch ctx.Message().(type) {
					case int, string, bool:
						observed++
					case float64:
						ctx.Reply(observed)
					}
				}))
				require.NoError(t, err)
				observerContext := &eventStreamContext{ref: observer, logger: &eventStreamLogger{}}
				for _, event := range []any{0, "", true} {
					stream.Subscribe(observerContext, event)
				}
				launched := make(chan *Context, 1)
				entered, release := make(chan struct{}), make(chan struct{})
				var released bool
				defer func() {
					if !released {
						close(release)
					}
				}()
				fail := func(ctx vivid.EventStreamContext) error {
					stream.Subscribe(ctx, true)
					close(entered)
					<-release
					if scenario == "provider_panic" || scenario == "restarted_panic" || scenario == "prelaunch_panic" {
						panic("restart initialization panic")
					}
					return context.Canceled
				}
				var cleaned bool
				ownerLogger := &eventStreamLogger{Logger: logger, observe: func(message string) {
					if message == "unsubscribed all events" {
						cleaned = true
						// 清理日志转发为事件时，不应重新选中失败 Actor。
						stream.Publish(system, 1)
					}
				}}
				var owner *Context
				provider := vivid.ActorProviderFN(func() vivid.Actor {
					if scenario == "provider" || scenario == "provider_panic" {
						_ = fail(owner)
						return nil
					}
					return vivid.NewComplexCombinationActor(
						vivid.NewRestartedActor(func(ctx vivid.RestartContext) error {
							if scenario == "restarted" || scenario == "restarted_panic" {
								return fail(ctx)
							}
							return nil
						}),
						vivid.NewPrelaunchActor(func(ctx vivid.PrelaunchContext) error { return fail(ctx) }),
					)
				})
				ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
					switch message := ctx.Message().(type) {
					case *vivid.OnLaunch:
						ctx.EventStream().Subscribe(ctx, 0)
						launched <- ctx.(*Context)
					case *vivid.OnKilled:
						if message.Ref.Equals(ctx.Ref()) {
							ctx.EventStream().Subscribe(ctx, "")
						}
					}
				}), vivid.WithActorName("owner"), vivid.WithActorProvider(provider), vivid.WithActorLogger(ownerLogger))
				require.NoError(t, err)
				owner = receiveRetryResult(t, launched)
				system.AdvancedTell(true, ref, &RestartMessage{Reason: "subscription cleanup"})
				receiveRetryResult(t, entered)
				requireEventStreamSubscriptions(t, stream, ref, 0, "", true)
				close(release)
				released = true
				// 框架 Ping 排在失败重启之后，避免读取中间状态。
				_, err = system.Ask(ref, new(messages.PingMessage), time.Second).Result()
				require.NoError(t, err)
				require.True(t, owner.zombie)
				require.False(t, system.HeartbeatProbe(ref).Available)
				require.True(t, cleaned)
				requireEventStreamSubscriptions(t, stream, ref)
				for _, event := range []any{1, "event", true} {
					stream.Publish(system, event)
				}
				result, err := system.Ask(observer, float64(0), time.Second).Result()
				require.NoError(t, err)
				require.Equal(t, 4, result, "失败清理不影响其他 Actor 的订阅及后续事件")
				if stop {
					require.NoError(t, system.Stop(time.Second))
				} else {
					system.Kill(ref, false)
					require.NoError(t, system.Stop(time.Second))
				}
				requireEventStreamSubscriptions(t, stream, ref)
			})
		}
	}
}

func BenchmarkEventStream_RestartFailure(b *testing.B) {
	system := NewSystem(vivid.WithActorSystemLogger(log.NewSilentLogger(log.WithLevel(log.LevelError))))
	require.NoError(b, system.Start())
	b.Cleanup(func() { require.NoError(b, system.Stop()) })
	launched, removed := make(chan struct{}, 1), make(chan struct{}, 1)
	parent, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch message := ctx.Message().(type) {
		case bool:
			ref, err := ctx.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
				if _, ok := ctx.Message().(*vivid.OnLaunch); ok {
					ctx.EventStream().Subscribe(ctx, 0)
					launched <- struct{}{}
				}
			}), vivid.WithActorName("child"), vivid.WithActorProvider(vivid.ActorProviderFN(func() vivid.Actor { return nil })))
			if err != nil {
				ctx.Reply(err)
			} else {
				ctx.Reply(ref)
			}
		case *vivid.OnKilled:
			if !message.Ref.Equals(ctx.Ref()) {
				removed <- struct{}{}
			}
		}
	}))
	require.NoError(b, err)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		result, err := system.Ask(parent, true, time.Second).Result()
		if err != nil {
			b.Fatalf("create actor: %v, %v", result, err)
		}
		ref, ok := result.(vivid.ActorRef)
		if !ok {
			b.Fatalf("create actor: %v", result)
		}
		<-launched
		system.AdvancedTell(true, ref, &RestartMessage{Reason: "benchmark failure cleanup"})
		if _, err := system.Ask(ref, new(messages.PingMessage), time.Second).Result(); err != nil {
			b.Fatal(err)
		}
		system.Kill(ref, false)
		<-removed
	}
	b.StopTimer()
}

func TestEventStream_RestartSuccess(t *testing.T) {
	for _, replace := range []bool{false, true} {
		name := "reuse"
		if replace {
			name = "provider"
		}
		t.Run(name, func(t *testing.T) {
			system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
			t.Cleanup(func() { _ = system.Stop(time.Second) })
			stream := system.eventStream.(*eventStream)
			launched := make(chan *Context, 2)
			var count int
			behavior := vivid.ActorFN(func(ctx vivid.ActorContext) {
				switch message := ctx.Message().(type) {
				case *vivid.OnLaunch:
					launched <- ctx.(*Context)
				case *vivid.OnKilled:
					if message.Ref.Equals(ctx.Ref()) {
						ctx.EventStream().Subscribe(ctx, "")
					}
				case int, bool, string:
					count++
				case float64:
					ctx.Reply(count)
				}
			})
			actor := vivid.NewComplexCombinationActor(
				vivid.NewRestartedActor(func(ctx vivid.RestartContext) error {
					stream.Subscribe(ctx, true)
					return nil
				}), behavior,
			)
			var options []vivid.ActorOption
			if replace {
				options = append(options, vivid.WithActorProvider(vivid.ActorProviderFN(func() vivid.Actor { return actor })))
			}
			ref, err := system.ActorOf(behavior, options...)
			require.NoError(t, err)
			owner := receiveRetryResult(t, launched)
			stream.Subscribe(owner, 0)
			system.AdvancedTell(true, ref, &RestartMessage{Reason: "subscription retention"})
			require.Same(t, owner, receiveRetryResult(t, launched))
			if replace {
				requireEventStreamSubscriptions(t, stream, ref, 0, "", true)
			} else {
				requireEventStreamSubscriptions(t, stream, ref, 0, "")
			}
			stream.Publish(system, 1)
			stream.Publish(system, "event")
			if replace {
				stream.Publish(system, true)
			}
			result, err := system.Ask(ref, float64(0), time.Second).Result()
			require.NoError(t, err)
			expected := 2
			if replace {
				expected++
			}
			require.Equal(t, expected, result)
			require.NoError(t, system.Stop(time.Second))
			requireEventStreamSubscriptions(t, stream, ref)
		})
	}
}
