package actor

import (
	"sync"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/future"
	"github.com/kercylan98/vivid/internal/guard"
	"github.com/kercylan98/vivid/internal/transparent"
	"github.com/kercylan98/vivid/pkg/result"
)

var (
	_ vivid.ActorSystem = (*System)(nil)
)

func NewSystem(options ...vivid.ActorSystemOption) *result.Result[*System] {
	opts := &vivid.ActorSystemOptions{
		DefaultAskTimeout: vivid.DefaultAskTimeout,
	}
	for _, option := range options {
		option(opts)
	}

	system := &System{
		options:           opts,
		actorContexts:     sync.Map{},
		guardClosedSignal: make(chan struct{}),
	}

	var err error
	system.Context, err = NewContext(system, nil, guard.NewActor(system.guardClosedSignal))
	if err != nil {
		return result.Error[*System](err)
	}
	return result.With(system, nil)
}

type System struct {
	*Context          // ActorSystem 本身就表示了根 Actor
	options           *vivid.ActorSystemOptions
	actorContexts     sync.Map // 用于加速访问的 ActorContext 缓存（含有 Future）
	guardClosedSignal chan struct{}
}

func (s *System) Stop() {
	s.Context.Kill(s.Context.Ref(), true, "actor system stop")
	<-s.guardClosedSignal
}

func (s *System) appendFuture(agentRef *AgentRef, future *future.Future[vivid.Message]) {
	s.actorContexts.Store(agentRef.ref.GetPath(), future)
}

func (s *System) removeFuture(agentRef *AgentRef) {
	s.actorContexts.Delete(agentRef.ref.GetPath())
}

// appendActorContext 用于添加指定路径的 ActorContext。
func (s *System) appendActorContext(ctx *Context) bool {
	_, loaded := s.actorContexts.LoadOrStore(ctx.Ref().GetPath(), ctx)
	return loaded
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
