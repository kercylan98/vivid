package cluster

import (
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/mailbox"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
)

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

// SingletonProxyActorName 集群单例代理 Manager 在根下的 Actor 名称。
const SingletonProxyActorName = "@cluster-singleton-proxies"

// NewSingletonProxy 创建集群单例代理 Actor，用于将消息转发到当前 Leader 上的单例。
// 代理会订阅 ClusterLeaderChangedEvent，在 Leader 变更时更新缓存的单例 ref；无可用 ref 时缓冲消息，待 ref 恢复后转发。
// 业务通过 ClusterContext.SingletonRef(name) 获取代理 ref，再向代理 Tell 消息即可，代理会转发到当前单例（单例侧看到的 sender 为代理）。
func NewSingletonProxy(name string, rawSenderGetter func(ctx vivid.ActorContext) vivid.ActorRef) vivid.Actor {
	return &singletonProxy{
		name:            name,
		rawSenderGetter: rawSenderGetter,
		bufferMessages:  make([]vivid.Message, 0, 16),
		bufferSenders:   make([]vivid.ActorRef, 0, 16),
	}
}

type singletonProxy struct {
	name            string
	rawSenderGetter func(ctx vivid.ActorContext) vivid.ActorRef
	cachedRef       vivid.ActorRef
	bufferSenders   []vivid.ActorRef
	bufferMessages  []vivid.Message
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
		ctx.Logger().Info("singleton leader changed", log.String("name", p.name), log.Any("error", err))
		return
	}
	oldRef := p.cachedRef
	p.updateCachedRef(ctx, ref)

	if ref != nil && ref != oldRef {
		ctx.Logger().Info("singleton leader changed", log.String("name", p.name), log.Any("oldRef", oldRef.String()), log.Any("newRef", ref.String()))
	}
}

func (p *singletonProxy) updateCachedRef(ctx vivid.ActorContext, ref vivid.ActorRef) {
	p.cachedRef = ref
	// 若有新 ref 并且有缓冲消息，flush
	if ref != nil && len(p.bufferMessages) > 0 {
		bufferedMessages := p.bufferMessages
		bufferedSenders := p.bufferSenders

		// 重置缓冲区
		p.bufferMessages = make([]vivid.Message, 0, 16)
		p.bufferSenders = make([]vivid.ActorRef, 0, 16)

		// 转发缓冲的消息
		for i := range bufferedMessages {
			p.forward(ctx, ref, bufferedSenders[i], bufferedMessages[i])
		}
	}
}

func (p *singletonProxy) forwardOrBuffer(ctx vivid.ActorContext, msg vivid.Message) {
	ref := p.cachedRef
	if ref == nil {
		p.bufferMessages = append(p.bufferMessages, msg)
		p.bufferSenders = append(p.bufferSenders, ctx.Sender())
		ctx.Logger().Info("singleton proxy standby", log.String("name", p.name), log.Any("sender", ctx.Sender().String()), log.Any("message", msg))
		return
	}
	p.forward(ctx, ref, ctx.Sender(), msg)
}

func (p *singletonProxy) forward(ctx vivid.ActorContext, cachedRef vivid.ActorRef, sender vivid.ActorRef, msg vivid.Message) {
	ctx.Tell(cachedRef, mailbox.NewEnvelop(false, p.rawSenderGetter(ctx), nil, msg).WithAgent(sender))
	ctx.Logger().Info("singleton proxy", log.String("name", p.name), log.Any("sender", sender.String()), log.Any("message", msg))
}
