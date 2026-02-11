package cluster

import (
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
)

const singletonKillReasonNotLeader = "cluster singleton stopped: not leader or not in quorum"

var _ vivid.Actor = (*singletonManager)(nil)

// SingletonsActorName 集群单例 Manager 在根下的 Actor 名称，路径为 ClusterSingletonsPathPrefix。
const SingletonsActorName = "@cluster-singletons"

// NewSingletonManager 负责根据集群单例模板创建和管理 Singleton Manager Actor。
// 当启用集群且配置了单例模板时，系统会将其挂载到根（名称为 SingletonsActorName）。
// 传入的 templates 会被深拷贝，后续外部修改不会影响 Manager 内部状态。
// Manager 会订阅 ClusterLeaderChangedEvent，仅当前节点既为 Leader 且处于法定派时，才会创建对应的单例子 Actor；否则会移除所有已创建的子 Actor。
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
	templates map[string]vivid.ActorProvider // 集群单例模板，用于创建集群单例 Actor。
	children  map[string]vivid.ActorRef      // 集群单例 Actor 引用，用于管理集群单例 Actor。
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
		ctx.Logger().Error("cluster singleton manager: cluster view unavailable", log.Any("error", err))
		return
	}
	m.createSingletons(ctx, view.LeaderAddr == ctx.Ref().GetAddress(), view.InQuorum)
}

func (m *singletonManager) onLeaderChanged(ctx vivid.ActorContext, ev ves.ClusterLeaderChangedEvent) {
	m.createSingletons(ctx, ev.IAmLeader, ev.InQuorum)
}

func (m *singletonManager) createSingletons(ctx vivid.ActorContext, isLeader bool, inQuorum bool) {
	if isLeader && inQuorum {
		var started []string
		for name, provider := range m.templates {
			if m.children[name] != nil {
				continue
			}
			child, err := ctx.ActorOf(m.singletonActor(provider), vivid.WithActorName(name))
			if err != nil {
				ctx.Logger().Warn("cluster singleton manager: spawn failed",
					log.String("singleton", name),
					log.Any("error", err))
				continue
			}
			m.children[name] = child
			started = append(started, name)
		}
		if len(started) > 0 {
			ctx.Logger().Debug("cluster singleton manager: singletons started", log.Any("singletons", started))
		}
		return
	}

	if len(m.children) == 0 {
		return
	}
	names := make([]string, 0, len(m.children))
	for name, ref := range m.children {
		if ref != nil {
			ctx.Kill(ref, false, singletonKillReasonNotLeader)
			names = append(names, name)
		}
		delete(m.children, name)
	}
	ctx.Logger().Debug("cluster singleton manager: singletons stopped", log.String("reason", singletonKillReasonNotLeader), log.Any("stopped", names))
}

func (m *singletonManager) singletonActor(provider vivid.ActorProvider) vivid.Actor {
	actor := provider.Provide()
	return vivid.ActorFN(func(ctx vivid.ActorContext) {
		ctx = newSingletonActorContext(ctx)
		actor.OnReceive(ctx)
	})
}
