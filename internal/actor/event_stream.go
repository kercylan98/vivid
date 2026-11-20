package actor

import (
	"maps"
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/log"
)

var (
	_ vivid.EventStream = (*eventStream)(nil)
)

func newEventStream(system *System) *eventStream {
	return &eventStream{
		system:          system,
		subscribers:     make(map[reflect.Type]map[string]vivid.ActorRef),
		subscriberTypes: make(map[string]map[reflect.Type]struct{}),
	}
}

type eventStream struct {
	system          *System                                    // 系统
	subscribers     map[reflect.Type]map[string]vivid.ActorRef // 事件类型 -> 订阅者 Path -> 订阅者 ActorRef
	subscriberTypes map[string]map[reflect.Type]struct{}       // 订阅者 Path -> 订阅者订阅的事件类型
	mu              sync.RWMutex
}

type eventStreamScope struct{ marker byte }

type eventStreamLifetime struct {
	scope  *eventStreamScope
	closed atomic.Bool
}

// closeEventStream 先关闭准入，再清理已接纳的登记；迟到订阅不能越过清理快照。
func (c *Context) closeEventStream() {
	c.ref.(*Ref).events.closed.Store(true)
	c.EventStream().UnsubscribeAll(c)
}

func sameEventStreamSubscriber(current, expected vivid.ActorRef) bool {
	left, leftOK := current.(*Ref)
	right, rightOK := expected.(*Ref)
	if leftOK && rightOK && left.events != nil {
		return left.events == right.events
	}
	return leftOK && rightOK && left == right
}

func (es *eventStream) Subscribe(ctx vivid.EventStreamContext, event any) {
	eventType := reflect.TypeOf(event)
	if eventType == nil {
		panic(vivid.ErrorIllegalArgument.WithMessage("event type is required"))
	}
	subscriber := ctx.Ref()
	subscriberPath := subscriber.GetPath()

	es.mu.Lock()
	if es.system != nil {
		ref, ok := subscriber.(*Ref)
		if !ok || ref.events == nil || ref.events.scope != es.system.eventStreamScope || ref.events.closed.Load() {
			es.mu.Unlock()
			panic(vivid.ErrorActorDeaded)
		}
	}

	subscribers, ok := es.subscribers[eventType]
	if !ok {
		subscribers = make(map[string]vivid.ActorRef)
		es.subscribers[eventType] = subscribers
	}
	if _, ok = subscribers[subscriberPath]; ok {
		es.mu.Unlock()
		ctx.Logger().Warn("event already subscribed", log.String("event_type", eventType.String()), log.String("subscriber_path", subscriberPath))
		return
	}
	subscribers[subscriberPath] = subscriber

	types, ok := es.subscriberTypes[subscriberPath]
	if !ok {
		types = make(map[reflect.Type]struct{})
		es.subscriberTypes[subscriberPath] = types
	}
	types[eventType] = struct{}{}
	es.mu.Unlock()

	// 日志器可重入事件流，订阅表更新完成后再调用。
	ctx.Logger().Debug("event subscribed", log.String("event_type", eventType.String()), log.String("subscriber_path", subscriberPath))
}

func (es *eventStream) Publish(ctx vivid.EventStreamContext, event vivid.Message) {
	eventType := reflect.TypeOf(event)
	if eventType == nil {
		panic(vivid.ErrorIllegalArgument.WithMessage("event type is required"))
	}

	es.mu.RLock()
	subscribers := maps.Clone(es.subscribers[eventType])
	es.mu.RUnlock()

	for _, subscriber := range subscribers {
		es.system.tell(false, subscriber, vivid.StreamEvent(event))
	}

	//ctx.Logger().Debug("event published", log.String("event_type", eventType.String()), log.String("publisher_path", ctx.Ref().GetPath()), log.Any("event_data", event))
}

func (es *eventStream) Unsubscribe(ctx vivid.EventStreamContext, event any) {
	eventType := reflect.TypeOf(event)
	if eventType == nil {
		panic(vivid.ErrorIllegalArgument.WithMessage("event type is required"))
	}
	subscriber := ctx.Ref()
	subscriberPath := subscriber.GetPath()

	es.mu.Lock()

	subscribers, ok := es.subscribers[eventType]
	if !ok {
		es.mu.Unlock()
		ctx.Logger().Debug("event not subscribed", log.String("subscriber_path", subscriberPath), log.String("event_type", eventType.String()))
		return
	}
	if current, exists := subscribers[subscriberPath]; exists && !sameEventStreamSubscriber(current, subscriber) {
		es.mu.Unlock()
		return
	}

	delete(subscribers, subscriberPath)
	delete(es.subscriberTypes[subscriberPath], eventType)
	if len(es.subscriberTypes[subscriberPath]) == 0 {
		delete(es.subscriberTypes, subscriberPath)
	}
	es.mu.Unlock()

	ctx.Logger().Debug("event unsubscribed", log.String("subscriber_path", subscriberPath), log.String("event_type", eventType.String()))
}

func (es *eventStream) UnsubscribeAll(ctx vivid.EventStreamContext) {
	subscriber := ctx.Ref()
	subscriberPath := subscriber.GetPath()
	es.mu.Lock()

	count := len(es.subscriberTypes[subscriberPath])
	if count == 0 {
		es.mu.Unlock()
		return
	}
	// 同路径可能已经属于新实例，旧上下文的迟到清理不能删除新登记。
	for eventType := range es.subscriberTypes[subscriberPath] {
		if !sameEventStreamSubscriber(es.subscribers[eventType][subscriberPath], subscriber) {
			es.mu.Unlock()
			return
		}
		break
	}

	for eventType := range es.subscriberTypes[subscriberPath] {
		delete(es.subscribers[eventType], subscriberPath)
		if len(es.subscribers[eventType]) == 0 {
			delete(es.subscribers, eventType)
		}
	}
	delete(es.subscriberTypes, subscriberPath)
	es.mu.Unlock()

	ctx.Logger().Debug("unsubscribed all events", log.String("subscriber_path", subscriberPath), log.Int("count", count))
}
