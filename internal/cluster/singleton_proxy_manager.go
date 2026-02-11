package cluster

import (
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/log"
)

var _ vivid.Actor = (*singletonProxyManager)(nil)

// NewSingletonProxyManager 创建集群单例代理管理器，按 name 按需创建代理子 Actor。
// 由系统在启用集群时挂载到根下（名称 SingletonProxyActorName）。
func NewSingletonProxyManager() vivid.Actor {
	return &singletonProxyManager{
		children: make(map[string]vivid.ActorRef),
	}
}

type singletonProxyManager struct {
	children map[string]vivid.ActorRef
}

func (m *singletonProxyManager) OnReceive(ctx vivid.ActorContext) {
	switch msg := ctx.Message().(type) {
	case *GetOrCreateProxyRequest:
		m.onGetOrCreateProxyRequest(ctx, msg)
	default:
		ctx.Reply(&GetOrCreateProxyResponse{Err: vivid.ErrorIllegalArgument})
	}
}

func (m *singletonProxyManager) onGetOrCreateProxyRequest(ctx vivid.ActorContext, msg *GetOrCreateProxyRequest) {
	name := msg.Name
	if name == "" {
		ctx.Reply(&GetOrCreateProxyResponse{Err: vivid.ErrorIllegalArgument})
		return
	}
	ref, exists := m.children[name]
	if exists {
		ctx.Reply(&GetOrCreateProxyResponse{Ref: ref})
		return
	}

	proxy := NewSingletonProxy(name)
	ref, err := ctx.ActorOf(proxy, vivid.WithActorName(name))
	if err != nil {
		ctx.Reply(&GetOrCreateProxyResponse{Err: err})
		return
	}
	m.children[name] = ref

	ctx.Reply(&GetOrCreateProxyResponse{Ref: ref})

	ctx.Logger().Debug("singleton proxy generated", log.String("name", name), log.Any("ref", ref.String()))
}
