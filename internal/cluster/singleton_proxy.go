package cluster

import (
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
)

// SingletonProxyActorName 集群单例代理 Manager 在根下的 Actor 名称。
const SingletonProxyActorName = "@cluster-singleton-proxies"

var _ vivid.Actor = (*singletonProxy)(nil)

// GetOrCreateProxyRequest 向 ProxyManager 请求获取或创建指定名称的单例代理，仅内部使用。
type GetOrCreateProxyRequest struct {
	Name string
}

// GetOrCreateProxyResponse 返回单例代理的 ActorRef 或错误，仅内部使用。
type GetOrCreateProxyResponse struct {
	Ref vivid.ActorRef
	Err error
}

// NewSingletonProxy 创建集群单例代理 Actor，用于将消息转发到当前 Leader 上的单例。
// 代理会订阅 ClusterLeaderChangedEvent，在 Leader 变更时更新缓存的单例 ref；无可用 ref 时缓冲消息，待 ref 恢复后转发。
// 业务通过 ClusterContext.SingletonRef(name) 获取代理 ref，再向代理 Tell 消息即可，代理会转发到当前单例（单例侧看到的 sender 为代理）。
func NewSingletonProxy(name string) vivid.Actor {
	return &singletonProxy{
		name:   name,
		buffer: make([]*singletonForwardedMessage, 0, 16),
	}
}

type singletonProxy struct {
	name      string
	cachedRef vivid.ActorRef
	buffer    []*singletonForwardedMessage
}

func (p *singletonProxy) OnReceive(ctx vivid.ActorContext) {
	switch ev := ctx.Message().(type) {
	case *vivid.OnLaunch:
		p.onLaunch(ctx)
	case ves.ClusterLeaderChangedEvent:
		p.onLeaderChanged(ctx, ev)
	default:
		p.forwardOrBuffer(ctx, ev)
	}
}

func (p *singletonProxy) onLaunch(ctx vivid.ActorContext) {
	ctx.EventStream().Subscribe(ctx, ves.ClusterLeaderChangedEvent{})
	view, err := ctx.Cluster().GetView()
	if err != nil {
		ctx.Logger().Error("failed to get cluster view", log.Any("error", err))
		return
	}
	ref, err := ctx.System().CreateRef(view.LeaderAddr, ClusterSingletonsPathPrefix+"/"+p.name)
	if err != nil {
		ctx.Logger().Error("failed to create singleton ref", log.Any("error", err))
		return
	}
	p.updateCachedRef(ctx, ref)
}

func (p *singletonProxy) onLeaderChanged(ctx vivid.ActorContext, e ves.ClusterLeaderChangedEvent) {
	ref, err := ctx.System().CreateRef(e.LeaderAddr, ClusterSingletonsPathPrefix+"/"+p.name)
	if err != nil {
		p.cachedRef = nil
		ctx.Logger().Debug("singleton leader changed", log.String("name", p.name), log.Any("error", err))
		return
	}
	oldRef := p.cachedRef
	p.updateCachedRef(ctx, ref)

	if ref != nil && ref != oldRef {
		ctx.Logger().Debug("singleton leader changed", log.String("name", p.name), log.Any("oldRef", oldRef.String()), log.Any("newRef", ref.String()))
	}
}

func (p *singletonProxy) updateCachedRef(ctx vivid.ActorContext, ref vivid.ActorRef) {
	p.cachedRef = ref
	// 若有新 ref 并且有缓冲消息，flush
	if ref != nil && len(p.buffer) > 0 {
		bufferedMessages := p.buffer
		p.buffer = make([]*singletonForwardedMessage, 0, 16)
		// 转发缓冲的消息
		for _, message := range bufferedMessages {
			p.forward(ctx, message)
		}
	}
}

func (p *singletonProxy) forwardOrBuffer(ctx vivid.ActorContext, msg vivid.Message) {
	message := &singletonForwardedMessage{
		sender:  ctx.Sender(),
		message: msg,
	}

	if p.cachedRef == nil {
		p.buffer = append(p.buffer, message)
		ctx.Logger().Debug("singleton proxy standby", log.String("name", p.name), log.Any("sender", ctx.Sender().String()), log.Any("message", msg))
		return
	}

	p.forward(ctx, message)
}

func (p *singletonProxy) forward(ctx vivid.ActorContext, message *singletonForwardedMessage) {
	ctx.Tell(p.cachedRef, message)
	ctx.Logger().Debug("singleton proxy", log.String("name", p.name), log.Any("sender", message.sender.String()), log.Any("message", message.message))
}
