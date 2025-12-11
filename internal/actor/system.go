package actor

import (
	"sync"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/future"
	"github.com/kercylan98/vivid/internal/guard"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/result"
)

var (
	_ vivid.ActorSystem = (*System)(nil)
)

func NewSystem(options ...vivid.ActorSystemOption) *result.Result[*System] {
	opts := &vivid.ActorSystemOptions{
		DefaultAskTimeout: vivid.DefaultAskTimeout,
		Logger:            log.GetDefault(),
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

// findMailbox 负责根据给定的 ActorRef 查找并返回对应的邮箱（Mailbox）。
func (s *System) findMailbox(ref *Ref) vivid.Mailbox {
	if ref == nil {
		// 若传入的引用为 nil，直接返回系统根 Actor 的邮箱作为兜底。
		return s.Mailbox()
	}

	// 尝试优先从 Ref 的 cache 字段中读取 Mailbox 指针，如果存在则直接返回，减少 map 查找的开销。
	if ptr := ref.cache.Load(); ptr != nil {
		return *ptr
	}

	// 当前仅支持本地地址查找，若 ref 非本地地址则直接 panic，待实现远程消息转发逻辑。
	if ref.GetAddress().String() != s.Ref().GetAddress().String() {
		panic("findMailbox: remote ref lookup not implemented")
	}

	// 在 actorContexts 中查找指定路径（GetPath）对应的 Context，并尝试获取其邮箱（Mailbox）。
	if value, ok := s.actorContexts.Load(ref.GetPath()); ok {
		if ctx, ok := value.(*Context); ok {
			mailbox := ctx.Mailbox()
			// 利用 CompareAndSwap 保证仅存储一次 Mailbox 指针到 cache，提升后续命中率，防止多线程下的闭包问题。
			ref.cache.CompareAndSwap(nil, &mailbox)
			return mailbox
		}
	}
	// 若上述皆未命中，返回系统根 Actor 的 Mailbox 作为默认兜底方案，保证 Mailbox 一定可用。
	return s.Mailbox()
}
