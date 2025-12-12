package actor

import (
	"sync"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/future"
	"github.com/kercylan98/vivid/internal/guard"
	"github.com/kercylan98/vivid/internal/remoting"
	"github.com/kercylan98/vivid/pkg/sugar"
)

var (
	_ vivid.ActorSystem = (*System)(nil)
)

func NewSystem(options ...vivid.ActorSystemOption) *sugar.Result[*System] {
	opts := vivid.NewActorSystemOptions(options...)

	system := &System{
		options:           opts,
		actorContexts:     sync.Map{},
		guardClosedSignal: make(chan struct{}),
	}

	var err error
	system.Context, err = NewContext(system, nil, guard.NewActor(system.guardClosedSignal))
	if err != nil {
		return sugar.Err[*System](err)
	}

	// 初始化远程服务器 Actor
	if opts.RemotingBindAddress != "" && opts.RemotingAdvertiseAddress != "" {
		system.ActorOf(remoting.NewServerActor(opts.RemotingBindAddress, opts.RemotingAdvertiseAddress),
			vivid.WithActorName("@remoting"),
		)
	}

	return sugar.With(system, nil)
}

type System struct {
	*Context          // ActorSystem 本身就表示了根 Actor
	options           *vivid.ActorSystemOptions
	actorContexts     sync.Map // 用于加速访问的 ActorContext 缓存（含有 Future）
	guardClosedSignal chan struct{}
}

func (s *System) HandleEnvelop(envelop vivid.Envelop) {
	receiverMailbox := s.findMailbox(envelop.Sender().(*Ref))
	receiverMailbox.Enqueue(envelop)
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

	// 检查是否为远程地址，使用远程邮箱
	if ref.GetAddress() != s.Ref().GetAddress() {
		return s.getOrCreateRemoteMailbox(ref.address)
	}

	// 在 actorContexts 中查找指定路径（GetPath）对应的 Context，并尝试获取其邮箱（Mailbox）。
	if value, ok := s.actorContexts.Load(ref.GetPath()); ok {
		switch v := value.(type) {
		case *Context:
			mailbox := v.Mailbox()
			// 利用 CompareAndSwap 保证仅存储一次 Mailbox 指针到 cache，提升后续命中率，防止多线程下的闭包问题。
			ref.cache.CompareAndSwap(nil, &mailbox)
			return mailbox
		case *future.Future[vivid.Message]:
			return v
		}
	}
	// 若上述皆未命中，返回系统根 Actor 的 Mailbox 作为默认兜底方案，保证 Mailbox 一定可用。
	return s.Mailbox()
}

// getOrCreateRemoteMailbox 获取或创建远程邮箱
func (s *System) getOrCreateRemoteMailbox(advertiseAddr string) vivid.Mailbox {
	return nil
}
