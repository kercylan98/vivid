package vivid

import (
	"time"

	"github.com/kercylan98/vivid/pkg/vivid/internal/processor"
)

type RouterConfig struct {
	PoolSize       int            // 作为路由器的 Actor 池大小
	RouterSelector RouterSelector // 选择器
}

func newRouterActorProvider(config *RouterConfig, provider ActorProvider, actorConfig *ActorConfiguration) ActorProvider {
	return ActorProviderFN(func() Actor {
		ra := &routerActor{
			routerConfig: *config,
			config:       actorConfig,
			provider: ActorProviderFN(func() Actor {
				return newRouterMonitorActor(provider.Provide())
			}),
		}
		return ra
	})
}

type routerActor struct {
	routerConfig RouterConfig
	config       *ActorConfiguration
	provider     ActorProvider
	routerTable  *routerTable
}

func (r *routerActor) Receive(context ActorContext) {
	switch m := context.Message().(type) {
	case *OnLaunch:
		r.onLaunch(context, m)
	case *OnKill, *OnPreRestart, *OnRestart, *OnWatchEnd:
	case *RouterMetrics: // 将路由成员指标更新进行合并
		r.onRouterMetrics(context, m)
	case *OnKilled:
		r.onKilled(context, m)
	default:
		r.onBalance(context, m)
	}
}

func (r *routerActor) onLaunch(context ActorContext, m *OnLaunch) {
	var refs = make([]ActorRef, r.routerConfig.PoolSize)
	for i := 0; i < r.routerConfig.PoolSize; i++ {
		c := *r.config
		c.Name = ""
		refs[i] = context.ActorOfP(r.provider).FromConfig(&c)
	}

	r.routerTable = newRouterTable(refs)
}

func (r *routerActor) onBalance(context ActorContext, m Message) {
	if sender := context.Sender(); sender != nil {
		m = processor.WrapMessage(sender, m)
	}

	currentTime := time.Now()
	for _, ref := range r.routerConfig.RouterSelector.Balance(r.routerTable) {
		context.Tell(ref, m)
		if metrics, ok := r.routerTable.metrics[ref]; ok {
			metrics.MessageNum++
			metrics.LastBalanceTime = currentTime
		}
	}
}

func (r *routerActor) onRouterMetrics(context ActorContext, m *RouterMetrics) {
	if c, exist := r.routerTable.metrics[context.Sender()]; exist {
		c.merge(m)
		m.reset()
		metricsPool.Put(m)
	}
}

func (r *routerActor) onKilled(context ActorContext, m *OnKilled) {
	r.routerTable.remove(m.ref)
	if context.ChildNum() == 0 {
		context.Kill(context.Ref(), "natural")
	}
}
