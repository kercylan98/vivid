package actor

import (
	"reflect"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/require"
)

func TestEventStream_NilEvent(t *testing.T) {
	for _, operation := range []struct {
		name  string
		apply func(*eventStream, vivid.EventStreamContext, vivid.Message)
	}{
		{"subscribe", (*eventStream).Subscribe},
		{"unsubscribe", (*eventStream).Unsubscribe},
		{"publish", (*eventStream).Publish},
	} {
		for _, populated := range []bool{false, true} {
			name := operation.name + "/empty"
			if populated {
				name = operation.name + "/subscribed"
			}
			t.Run(name, func(t *testing.T) {
				es := newEventStream(nil)
				ctx := &eventStreamContext{ref: &Ref{path: "/subscriber"}, logger: &eventStreamLogger{}}
				if populated {
					es.Subscribe(ctx, "")
				}
				// 参数拒绝不能先进入用户回调或修改现有登记。
				var refCalls, loggerCalls int
				ctx.onRef = func() { refCalls++ }
				ctx.onLogger = func() { loggerCalls++ }
				var caught any
				func() {
					defer func() { caught = recover() }()
					operation.apply(es, ctx, nil)
				}()
				err, ok := caught.(error)
				require.True(t, ok, "期望参数错误，实际 panic: %v", caught)
				require.ErrorIs(t, err, vivid.ErrorIllegalArgument)
				require.Contains(t, err.Error(), "event type is required")
				require.Zero(t, refCalls, "nil 事件不应读取订阅者")
				require.Zero(t, loggerCalls, "nil 事件不应调用日志器")
				require.NotContains(t, es.subscribers, nil)
				if populated {
					require.Len(t, es.subscribers, 1)
					require.Len(t, es.subscriberTypes, 1)
					require.Len(t, es.subscribers[reflect.TypeFor[string]()], 1)
					require.Equal(t, ctx.ref, es.subscribers[reflect.TypeFor[string]()]["/subscriber"])
					require.Equal(t, map[reflect.Type]struct{}{reflect.TypeFor[string](): {}}, es.subscriberTypes["/subscriber"])
				} else {
					require.Empty(t, es.subscribers)
					require.Empty(t, es.subscriberTypes)
				}
				ctx.onRef, ctx.onLogger = nil, nil
				es.UnsubscribeAll(ctx)
				es.Subscribe(ctx, true)
				es.Unsubscribe(ctx, true)
				require.Empty(t, es.subscriberTypes, "参数错误恢复后仍可正常操作")
			})
		}
	}
}

func TestEventStream_TypedNil(t *testing.T) {
	type event struct{ value int }
	type eventMap map[string]int
	type eventSlice []int
	type eventFunc func()
	type eventChannel chan int
	type observation struct {
		count     int
		eventType reflect.Type
		isNil     bool
	}
	for _, value := range []any{(*event)(nil), eventMap(nil), eventSlice(nil), eventFunc(nil), eventChannel(nil)} {
		t.Run(reflect.TypeOf(value).String(), func(t *testing.T) {
			system := NewTestSystem(t, vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))))
			t.Cleanup(func() { _ = system.Stop(time.Second) })
			var count int
			var eventType reflect.Type
			var isNil bool
			ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
				if _, ok := ctx.Message().(string); ok {
					ctx.Reply(observation{count, eventType, isNil})
				} else if reflect.TypeOf(ctx.Message()) == reflect.TypeOf(value) {
					count++
					eventType = reflect.TypeOf(ctx.Message())
					isNil = reflect.ValueOf(ctx.Message()).IsNil()
				}
			}))
			require.NoError(t, err)
			stream := system.eventStream.(*eventStream)
			ctx := &eventStreamContext{ref: ref, logger: &eventStreamLogger{}}
			stream.Subscribe(ctx, value)
			stream.Subscribe(ctx, value)
			stream.Publish(ctx, value)
			stream.Unsubscribe(ctx, value)
			stream.Publish(ctx, value)
			result, err := system.Ask(ref, "observed", time.Second).Result()
			require.NoError(t, err)
			require.Equal(t, observation{1, reflect.TypeOf(value), true}, result)
			stream.Subscribe(ctx, value)
			require.NoError(t, system.Stop(time.Second))
			require.Empty(t, stream.subscriberTypes)
		})
	}
}

func TestEventStream_ValueAndPointerTypes(t *testing.T) {
	type event struct{}
	es := newEventStream(nil)
	ctx := &eventStreamContext{ref: &Ref{path: "/subscriber"}, logger: &eventStreamLogger{}}
	es.Subscribe(ctx, event{})
	es.Subscribe(ctx, (*event)(nil))
	es.Unsubscribe(ctx, (*event)(nil))
	require.Contains(t, es.subscriberTypes["/subscriber"], reflect.TypeFor[event]())
	require.NotContains(t, es.subscriberTypes["/subscriber"], reflect.TypeFor[*event]())
	es.UnsubscribeAll(ctx)
	for _, subscribers := range es.subscribers {
		require.Empty(t, subscribers)
	}
	require.Empty(t, es.subscriberTypes)
}
