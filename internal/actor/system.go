package actor

import (
	"sync"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/future"
	"github.com/kercylan98/vivid/internal/guard"
	"github.com/kercylan98/vivid/internal/transparent"
)

var (
	_ vivid.ActorSystem = &System{}
)

func NewSystem(options ...vivid.ActorSystemOption) *System {
	opts := &vivid.ActorSystemOptions{
		DefaultAskTimeout: vivid.DefaultAskTimeout,
	}
	for _, option := range options {
		option(opts)
	}

	system := &System{
		options:       opts,
		actorContexts: sync.Map{},
	}

	system.Context = NewContext(system, nil, guard.NewGuardActor())
	return system
}

type System struct {
	*Context      // ActorSystem 本身就表示了根 Actor
	options       *vivid.ActorSystemOptions
	actorContexts sync.Map // 用于加速访问的 ActorContext 缓存（含有 Future）
}

func (s *System) appendFuture(agentRef *agentRef, future *future.Future[vivid.Message]) {
	s.actorContexts.Store(agentRef.ref.GetPath(), future)
}

func (s *System) removeFuture(agentRef *agentRef) {
	s.actorContexts.Delete(agentRef.ref.GetPath())
}

// appendActorContext 用于添加指定路径的 ActorContext。
func (s *System) appendActorContext(ctx *Context) {
	s.actorContexts.Store(ctx.Ref().GetPath(), ctx)
}

// removeActorContext 用于移除指定路径的 ActorContext。
func (s *System) removeActorContext(ctx *Context) {
	s.actorContexts.Delete(ctx.Ref().GetPath())
}

// findTransportActorContext 用于查找指定路径的透明传输 ActorContext，如果找不到则返回根 ActorContext。
func (s *System) findTransportActorContext(ref *Ref) transparent.TransportContext {
	if ref == nil {
		return s.Context
	}

	cache := ref.cache.Load()
	if cache != nil {
		return *cache
	}

	if ref.GetAddress().String() != s.Context.Ref().GetAddress().String() {
		return newRemoteRef(s, ref)
	}

	value, ok := s.actorContexts.Load(ref.GetPath())
	if !ok {
		return s.Context
	}

	ctx, ok := value.(transparent.TransportContext)
	if !ok {
		return s.Context
	}

	ref.cache.Store(&ctx)
	return ctx
}
