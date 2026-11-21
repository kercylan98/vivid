package actor

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/messages"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/require"
)

func requireEventStreamRejected(t *testing.T, stream vivid.EventStream, ctx vivid.EventStreamContext) {
	t.Helper()
	var caught any
	func() {
		defer func() { caught = recover() }()
		stream.Subscribe(ctx, 0)
	}()
	err, ok := caught.(error)
	require.True(t, ok, "期望订阅准入错误，实际 panic: %v", caught)
	require.ErrorIs(t, err, vivid.ErrorActorDeaded)
}

func TestEventStream_LifetimeClosed(t *testing.T) {
	for _, scenario := range []string{"kill", "stop", "rollback", "restart"} {
		t.Run(scenario, func(t *testing.T) {
			system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewSilentLogger()))
			t.Cleanup(func() { _ = system.Stop(time.Second) })
			stream := system.eventStream.(*eventStream)
			var owner *Context
			if scenario == "rollback" {
				_, err := system.ActorOf(vivid.NewPrelaunchActor(func(ctx vivid.PrelaunchContext) error {
					owner = ctx.(*Context)
					stream.Subscribe(ctx, 0)
					return context.Canceled
				}))
				require.ErrorIs(t, err, context.Canceled)
			} else {
				ready := make(chan *Context, 1)
				ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
					switch message := ctx.Message().(type) {
					case *vivid.OnLaunch:
						stream.Subscribe(ctx, 0)
						ready <- ctx.(*Context)
					case *vivid.OnKill, *vivid.OnKilled:
						if message, ok := message.(*vivid.OnKilled); ok && !message.Ref.Equals(ctx.Ref()) {
							return
						}
						stream.Subscribe(ctx, "")
					}
				}), vivid.WithActorProvider(vivid.ActorProviderFN(func() vivid.Actor { return nil })))
				require.NoError(t, err)
				owner = receiveRetryResult(t, ready)
				if scenario == "restart" {
					system.AdvancedTell(true, ref, &RestartMessage{Reason: "close subscriptions"})
					_, err = system.Ask(ref, new(messages.PingMessage), time.Second).Result()
					require.NoError(t, err)
				} else {
					if scenario == "kill" {
						system.Kill(ref, false)
					}
					require.NoError(t, system.Stop(time.Second))
				}
			}
			requireEventStreamSubscriptions(t, stream, owner.Ref())
			requireEventStreamRejected(t, stream, owner)
			clone := &eventStreamContext{ref: owner.Ref().Clone(), logger: &eventStreamLogger{}}
			requireEventStreamRejected(t, stream, clone)
			stream.Unsubscribe(clone, 0)
			stream.UnsubscribeAll(clone)
			requireEventStreamSubscriptions(t, stream, owner.Ref())
		})
	}
}

func TestEventStream_LifetimeRegistrationRace(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewSilentLogger()))
	t.Cleanup(func() { _ = system.Stop(time.Second) })
	ready := make(chan *Context, 1)
	ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		if _, ok := ctx.Message().(*vivid.OnLaunch); ok {
			ready <- ctx.(*Context)
		}
	}), vivid.WithActorProvider(vivid.ActorProviderFN(func() vivid.Actor { return nil })))
	require.NoError(t, err)
	owner := receiveRetryResult(t, ready)
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	ctx := &eventStreamContext{ref: ref, logger: &eventStreamLogger{}, onRef: func() {
		close(entered)
		<-release
	}}
	result := make(chan any, 1)
	go func() {
		var caught any
		defer func() { caught = recover(); result <- caught }()
		system.EventStream().Subscribe(ctx, 0)
	}()
	receiveRetryResult(t, entered)
	system.AdvancedTell(true, ref, &RestartMessage{Reason: "registration race"})
	_, err = system.Ask(ref, new(messages.PingMessage), time.Second).Result()
	require.NoError(t, err)
	releaseOnce.Do(func() { close(release) })
	caught := receiveRetryResult(t, result)
	registrationErr, ok := caught.(error)
	require.True(t, ok)
	require.ErrorIs(t, registrationErr, vivid.ErrorActorDeaded)
	requireEventStreamSubscriptions(t, system.eventStream.(*eventStream), owner.Ref())
}

func TestEventStream_LifetimeCleanupReentry(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewSilentLogger()))
	t.Cleanup(func() { _ = system.Stop(time.Second) })
	ready, attempted := make(chan *Context, 1), make(chan any, 1)
	var owner *Context
	logger := &eventStreamLogger{Logger: log.NewSilentLogger(), observe: func(message string) {
		if message == "unsubscribed all events" {
			var caught any
			func() {
				defer func() { caught = recover() }()
				system.EventStream().Subscribe(owner, 0)
			}()
			attempted <- caught
		}
	}}
	ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		if _, ok := ctx.Message().(*vivid.OnLaunch); ok {
			ctx.EventStream().Subscribe(ctx, 0)
			ready <- ctx.(*Context)
		}
	}), vivid.WithActorLogger(logger), vivid.WithActorProvider(vivid.ActorProviderFN(func() vivid.Actor { return nil })))
	require.NoError(t, err)
	owner = receiveRetryResult(t, ready)
	system.AdvancedTell(true, ref, &RestartMessage{Reason: "logger reentry"})
	_, err = system.Ask(ref, new(messages.PingMessage), time.Second).Result()
	require.NoError(t, err)
	caught := receiveRetryResult(t, attempted)
	registrationErr, ok := caught.(error)
	require.True(t, ok)
	require.ErrorIs(t, registrationErr, vivid.ErrorActorDeaded)
	requireEventStreamSubscriptions(t, system.eventStream.(*eventStream), ref)
}

func TestEventStream_LifetimeIdentity(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewSilentLogger()))
	other := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewSilentLogger()))
	t.Cleanup(func() { _ = system.Stop(time.Second); _ = other.Stop(time.Second) })
	ref, err := system.ActorOf(vivid.ActorFN(func(vivid.ActorContext) {}), vivid.WithActorName("same"))
	require.NoError(t, err)
	foreign, err := other.ActorOf(vivid.ActorFN(func(vivid.ActorContext) {}), vivid.WithActorName("same"))
	require.NoError(t, err)
	parsed, err := ParseRef(ref.String())
	require.NoError(t, err)
	created, err := NewRef(ref.GetAddress(), ref.GetPath())
	require.NoError(t, err)
	stream := system.EventStream()
	for _, invalid := range []vivid.ActorRef{foreign, foreign.Clone(), parsed, created} {
		requireEventStreamRejected(t, stream, &eventStreamContext{ref: invalid, logger: &eventStreamLogger{}})
	}
	ctx := &eventStreamContext{ref: ref, logger: &eventStreamLogger{}}
	clone := &eventStreamContext{ref: ref.Clone(), logger: &eventStreamLogger{}}
	stream.Subscribe(ctx, 0)
	stream.UnsubscribeAll(ctx)
	stream.Subscribe(ctx, 0)
	stream.Unsubscribe(clone, 0)
	requireEventStreamSubscriptions(t, system.eventStream.(*eventStream), ref)
	stream.Subscribe(clone, 0)
	stream.UnsubscribeAll(ctx)
	requireEventStreamSubscriptions(t, system.eventStream.(*eventStream), ref)
}

func TestEventStream_LifetimeSamePath(t *testing.T) {
	system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewSilentLogger()))
	t.Cleanup(func() { _ = system.Stop(time.Second) })
	stream := system.eventStream.(*eventStream)
	ready, killed := make(chan *Context, 2), make(chan struct{}, 2)
	system.eventStream = &startupEventStream{EventStream: stream, observe: func(event vivid.Message) {
		if event, ok := event.(ves.ActorKilledEvent); ok && event.ActorRef.GetPath() == "/reused" {
			killed <- struct{}{}
		}
	}}
	actor := vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case *vivid.OnLaunch:
			ctx.EventStream().Subscribe(ctx, 0)
			ready <- ctx.(*Context)
		case int:
			ctx.Reply("received")
		}
	})
	oldRef, err := system.ActorOf(actor, vivid.WithActorName("reused"))
	require.NoError(t, err)
	old := receiveRetryResult(t, ready)
	system.Kill(oldRef, false)
	receiveRetryResult(t, killed)
	ref, err := system.ActorOf(actor, vivid.WithActorName("reused"))
	require.NoError(t, err)
	current := receiveRetryResult(t, ready)
	require.NotSame(t, oldRef, ref)
	for _, stale := range []vivid.EventStreamContext{old, &eventStreamContext{ref: oldRef.Clone(), logger: &eventStreamLogger{}}} {
		requireEventStreamRejected(t, stream, stale)
		stream.Unsubscribe(stale, 0)
		requireEventStreamSubscriptions(t, stream, ref, 0)
		stream.UnsubscribeAll(stale)
		requireEventStreamSubscriptions(t, stream, ref, 0)
	}
	clone := &eventStreamContext{ref: ref.Clone(), logger: &eventStreamLogger{}}
	stream.Unsubscribe(clone, 0)
	requireEventStreamSubscriptions(t, stream, ref)
	stream.Subscribe(current, 0)
	requireEventStreamSubscriptions(t, stream, ref, 0)
	result, err := system.Ask(ref, 1, time.Second).Result()
	require.NoError(t, err)
	require.Equal(t, "received", result)
}

func BenchmarkEventStream_LocalSubscriptions(b *testing.B) {
	for _, parallel := range []bool{false, true} {
		b.Run(fmt.Sprintf("parallel=%t", parallel), func(b *testing.B) {
			system := NewSystem(vivid.WithActorSystemLogger(log.NewSilentLogger()))
			require.NoError(b, system.Start())
			b.Cleanup(func() { require.NoError(b, system.Stop()) })
			owners := make([]vivid.EventStreamContext, 128)
			for i := range owners {
				ref, err := system.ActorOf(vivid.ActorFN(func(vivid.ActorContext) {}))
				require.NoError(b, err)
				owners[i] = &eventStreamContext{ref: ref, logger: &eventStreamLogger{}}
			}
			stream := system.EventStream()
			b.ReportAllocs()
			b.ResetTimer()
			if parallel {
				var next sync.Mutex
				var index int
				b.RunParallel(func(pb *testing.PB) {
					next.Lock()
					ctx := owners[index%len(owners)]
					index++
					next.Unlock()
					for pb.Next() {
						stream.Subscribe(ctx, 0)
						stream.Unsubscribe(ctx, 0)
					}
				})
			} else {
				for b.Loop() {
					stream.Subscribe(owners[0], 0)
					stream.Unsubscribe(owners[0], 0)
				}
			}
		})
	}
}
