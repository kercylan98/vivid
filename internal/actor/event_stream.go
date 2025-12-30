package actor

import (
	"maps"
	"reflect"
	"sync"

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

func (es *eventStream) Subscribe(ctx vivid.ActorContext, event any) {
	eventType := reflect.TypeOf(event)

	es.mu.Lock()
	defer es.mu.Unlock()

	subscribers, ok := es.subscribers[eventType]
	if !ok {
		subscribers = make(map[string]vivid.ActorRef)
		es.subscribers[eventType] = subscribers
	}
	subscriberPath := ctx.Ref().GetPath()
	if _, ok := subscribers[subscriberPath]; ok {
		ctx.Logger().Debug("event already subscribed", log.String("event_type", eventType.String()), log.String("subscriber_path", subscriberPath))
		return
	}
	subscribers[subscriberPath] = ctx.Ref()

	types, ok := es.subscriberTypes[subscriberPath]
	if !ok {
		types = make(map[reflect.Type]struct{})
		es.subscriberTypes[subscriberPath] = types
	}
	types[eventType] = struct{}{}

	ctx.Logger().Debug("event subscribed", log.String("event_type", eventType.String()), log.String("subscriber_path", subscriberPath))
}

func (es *eventStream) Publish(ctx vivid.ActorContext, event vivid.Message) {
	eventType := reflect.TypeOf(event)

	es.mu.RLock()
	subscribers := maps.Clone(es.subscribers[eventType])
	es.mu.RUnlock()

	for _, subscriber := range subscribers {
		es.system.tell(false, subscriber, vivid.StreamEvent(event))
	}
}

func (es *eventStream) Unsubscribe(ctx vivid.ActorContext, event any) {
	eventType := reflect.TypeOf(event)

	es.mu.Lock()
	defer es.mu.Unlock()

	subscriberPath := ctx.Ref().GetPath()
	subscribers, ok := es.subscribers[eventType]
	if !ok {
		ctx.Logger().Debug("event not subscribed", log.String("subscriber_path", subscriberPath), log.String("event_type", eventType.String()))
		return
	}

	delete(subscribers, subscriberPath)
	delete(es.subscriberTypes[subscriberPath], eventType)
	if len(es.subscriberTypes[subscriberPath]) == 0 {
		delete(es.subscriberTypes, subscriberPath)
	}

	ctx.Logger().Debug("event unsubscribed", log.String("subscriber_path", subscriberPath), log.String("event_type", eventType.String()))
}

func (es *eventStream) UnsubscribeAll(ctx vivid.ActorContext) {
	es.mu.Lock()
	defer es.mu.Unlock()

	subscriberPath := ctx.Ref().GetPath()
	count := len(es.subscriberTypes[subscriberPath])
	if count == 0 {
		return
	}

	for eventType := range es.subscriberTypes[subscriberPath] {
		delete(es.subscribers[eventType], subscriberPath)
		if len(es.subscribers[eventType]) == 0 {
			delete(es.subscribers, eventType)
		}
	}
	delete(es.subscriberTypes, subscriberPath)

	ctx.Logger().Debug("unsubscribed all events", log.String("subscriber_path", subscriberPath), log.Int("count", count))
}
