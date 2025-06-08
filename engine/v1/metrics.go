package vivid

import (
	"github.com/kercylan98/vivid/engine/v1/metrics"
)

type ActorSystemMetrics interface {
}

func newActorSystemMetrics(manager metrics.Manager) *actorSystemMetrics {
	return &actorSystemMetrics{
		manager: manager,
	}
}

type actorSystemMetrics struct {
	HookCore
	manager metrics.Manager
}

func (m *actorSystemMetrics) hooks() []Hook {
	return []Hook{m}
}

func (m *actorSystemMetrics) OnActorLaunch(ctx ActorContext) {
	m.manager.GetMetricCollector().Counter("actor.alive", 1) // 当前存活的 Actor 数量
}

func (m *actorSystemMetrics) OnActorKill(ctx ActorContext, message *OnKill) {
	m.manager.GetMetricCollector().Counter("actor.alive", -1) // 当前存活的 Actor 数量
}
