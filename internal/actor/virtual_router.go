package actor

import (
	"container/list"
	"context"
	"errors"
	"reflect"
	"sync"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/clusterstore"
	"github.com/kercylan98/vivid/internal/virtual"
)

var (
	errVirtualRouteNotFound = errors.New("virtual actor ownership not found")
	errVirtualRouterBusy    = errors.New("virtual actor router concurrency limit exceeded")
)

type virtualRouteBackend interface {
	lookup(context.Context, virtualKey) (clusterstore.VirtualOwnership, bool, error)
	deliver(context.Context, clusterstore.VirtualOwnership, vivid.ActorRef, vivid.Message) (virtual.DeliveryStatus, error)
}

// virtualRouter 的缓存只提供联系线索，不能代替接收端对归属的多数派复核。
// 所有工作在调用方执行，不建立后台刷新或逐身份 goroutine。
type virtualRouter struct {
	backend virtualRouteBackend
	ctx     context.Context
	slots   chan struct{}
	limit   int
	mu      sync.Mutex
	cache   map[virtualKey]*virtualRouteHint
	recent  list.List
}

// virtualRouteHint 发布后不再修改归属；迟到失败只撤销实际使用的缓存观察。
type virtualRouteHint struct {
	key       virtualKey
	ownership clusterstore.VirtualOwnership
	element   *list.Element
}

func newVirtualRouter(client *virtualOwnershipClient, capacity, concurrency int) (*virtualRouter, error) {
	if client == nil || client.control == nil || capacity <= 0 || concurrency <= 0 {
		return nil, vivid.ErrorIllegalArgument
	}
	r := &virtualRouter{backend: client, ctx: client.control.system.options.Context, slots: make(chan struct{}, concurrency), limit: capacity, cache: make(map[virtualKey]*virtualRouteHint)}
	// 系统取消后释放缓存；不为每个缓存项安排定时任务。
	context.AfterFunc(r.ctx, func() {
		r.mu.Lock()
		clear(r.cache)
		r.recent.Init()
		r.mu.Unlock()
	})
	return r, nil
}

// route 只确认准入；总预算覆盖解析、初次投递和至多一次新代次重定位。
// 未知回执绝不重发；明确拒绝也不能向同一代次反复发送。
func (r *virtualRouter) route(ctx context.Context, key virtualKey, sender vivid.ActorRef, message vivid.Message) (virtual.DeliveryStatus, error) {
	if _, err := clusterstore.VirtualOwnershipKey(key.kind, key.name); err != nil {
		return 0, err
	}
	if sender == nil || reflect.ValueOf(sender).Kind() == reflect.Pointer && reflect.ValueOf(sender).IsNil() || sender.IsVirtual() {
		return 0, vivid.ErrorIllegalArgument
	}
	senderText := sender.String()
	if _, err := ParseRef(senderText); err != nil || len(senderText) > 4<<10 || !virtual.ValidDeliveryMessage(message) {
		return 0, virtual.ErrInvalidRoutedDelivery
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if r.ctx.Err() != nil {
		return 0, vivid.ErrorActorSystemStopped
	}
	select {
	case r.slots <- struct{}{}:
		defer func() { <-r.slots }()
	default:
		return 0, errVirtualRouterBusy
	}
	// 即使调用方没有期限，也只有一次有限等待，重定位不重置预算。
	var cancel context.CancelFunc
	if _, ok := ctx.Deadline(); ok {
		ctx, cancel = context.WithCancel(ctx)
	} else {
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
	}
	defer cancel()
	stop := context.AfterFunc(r.ctx, cancel)
	defer stop()
	hint, err := r.resolve(ctx, key)
	if err != nil {
		return 0, err
	}
	status, err := r.send(ctx, hint, sender, message)
	if err != nil || !relocatableDelivery(status) {
		return status, err
	}
	next, err := r.resolve(ctx, key)
	if err != nil {
		return status, err
	}
	if next.ownership.Generation <= hint.ownership.Generation {
		// 记录可能仍指向正在收尾或无法复核的实例，不轮询等待激活。
		r.invalidate(next)
		return status, nil
	}
	return r.send(ctx, next, sender, message)
}

func relocatableDelivery(status virtual.DeliveryStatus) bool {
	return status == virtual.DeliveryStale || status == virtual.DeliveryUnavailable || status == virtual.DeliveryStopped
}

func (r *virtualRouter) send(ctx context.Context, hint *virtualRouteHint, sender vivid.ActorRef, message vivid.Message) (virtual.DeliveryStatus, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	status, err := r.backend.deliver(ctx, hint.ownership, sender, message)
	if err != nil || relocatableDelivery(status) || status == virtual.DeliveryInvalid {
		r.invalidate(hint)
	}
	return status, err
}

func (r *virtualRouter) resolve(ctx context.Context, key virtualKey) (*virtualRouteHint, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	if err := ctx.Err(); err != nil {
		r.mu.Unlock()
		return nil, err
	}
	if r.ctx.Err() != nil {
		r.mu.Unlock()
		return nil, vivid.ErrorActorSystemStopped
	}
	hint := r.cache[key]
	if hint != nil && hint.ownership.Generation != 0 {
		r.recent.MoveToFront(hint.element)
		r.mu.Unlock()
		return hint, nil
	}
	if hint == nil {
		hint = &virtualRouteHint{key: key}
		hint.element = r.recent.PushFront(hint)
		r.cache[key] = hint
		if len(r.cache) > r.limit {
			oldest := r.recent.Back()
			delete(r.cache, oldest.Value.(*virtualRouteHint).key)
			r.recent.Remove(oldest)
		}
	}
	r.mu.Unlock()
	// 冷查询也受 route 的并发名额限制。不共享调用方的取消预算，避免一个慢查询阻塞其他调用。
	ownership, exists, err := r.backend.lookup(ctx, key)
	if err == nil {
		err = ctx.Err()
	}
	if err == nil && !exists {
		err = errVirtualRouteNotFound
	}
	if err == nil {
		if _, invalid := ownership.Release(); invalid != nil || ownership.Kind != key.kind || ownership.Name != key.name {
			err = clusterstore.ErrInvalidVirtualOwnership
		}
	}
	if err != nil {
		r.invalidate(hint)
		return nil, err
	}
	resolved := &virtualRouteHint{key: key, ownership: ownership}
	r.mu.Lock()
	// 槽位已被淘汰、失效或被另一查询更新时，迟到结果只供本次使用，不覆盖新缓存。
	if r.cache[key] == hint && ctx.Err() == nil && r.ctx.Err() == nil {
		resolved.element = hint.element
		resolved.element.Value = resolved
		r.cache[key] = resolved
		r.recent.MoveToFront(resolved.element)
	}
	r.mu.Unlock()
	return resolved, nil
}

func (r *virtualRouter) invalidate(hint *virtualRouteHint) {
	r.mu.Lock()
	if r.cache[hint.key] == hint {
		delete(r.cache, hint.key)
		r.recent.Remove(hint.element)
	}
	r.mu.Unlock()
}
