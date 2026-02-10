package cluster

import (
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
)

var _ vivid.Actor = (*singletonManager)(nil)

// SingletonsActorName 集群单例 Manager 在根下的 Actor 名称，路径为 ClusterSingletonsPathPrefix。
const SingletonsActorName = "@cluster-singletons"

// NewSingletonManager 根据集群单例模板创建 Manager Actor，由系统在启用集群且配置了单例模板时挂载到根下（名称 SingletonsActorName）。
// templates 会被拷贝，调用方后续修改不影响 Manager。Manager 订阅 ClusterLeaderChangedEvent，仅在 IAmLeader 且 InQuorum 时创建单例子 Actor，否则清理已创建的子 Actor。
func NewSingletonManager(templates map[string]vivid.ActorProvider) vivid.Actor {
	copy := make(map[string]vivid.ActorProvider, len(templates))
	for k, v := range templates {
		if v != nil {
			copy[k] = v
		}
	}
	return &singletonManager{
		templates: copy,
		children:  make(map[string]vivid.ActorRef),
	}
}

type singletonManager struct {
	templates map[string]vivid.ActorProvider
	children  map[string]vivid.ActorRef
}

func (m *singletonManager) OnReceive(ctx vivid.ActorContext) {
	switch ev := ctx.Message().(type) {
	case *vivid.OnLaunch:
		m.onLaunch(ctx)
	case ves.ClusterLeaderChangedEvent:
		m.onLeaderChanged(ctx, ev)
	}
}

func (m *singletonManager) onLaunch(ctx vivid.ActorContext) {
	ctx.EventStream().Subscribe(ctx, ves.ClusterLeaderChangedEvent{})
	view, err := ctx.Cluster().GetView()
	if err != nil {
		ctx.Logger().Error("failed to get cluster view", log.Any("error", err))
		return
	}
	m.createSingletons(ctx, view.LeaderAddr == ctx.Ref().GetAddress(), view.InQuorum)
}

func (m *singletonManager) onLeaderChanged(ctx vivid.ActorContext, ev ves.ClusterLeaderChangedEvent) {
	m.createSingletons(ctx, ev.IAmLeader, ev.InQuorum)
}

func (m *singletonManager) createSingletons(ctx vivid.ActorContext, isLeader bool, inQuorum bool) {
	if isLeader && inQuorum {
		for name, provider := range m.templates {
			if m.children[name] != nil {
				continue
			}
			child, err := ctx.ActorOf(provider.Provide(), vivid.WithActorName(name))
			if err != nil {
				ctx.Logger().Warn("cluster singleton spawn failed",
					log.String("name", name),
					log.Any("error", err))
				continue
			}
			m.children[name] = child
		}
		return
	}

	for name, ref := range m.children {
		if ref != nil {
			ctx.Kill(ref, false, "no longer leader or not in quorum")
		}
		delete(m.children, name)
	}
}
