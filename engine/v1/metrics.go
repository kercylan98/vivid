package vivid

import (
	"github.com/kercylan98/vivid/engine/v1/metrics"
)

type ActorSystemMetrics interface {
}

func newActorSystemMetrics(manager metrics.Manager) *actorSystemMetrics {
	return &actorSystemMetrics{
		Manager: manager,
	}
}

type actorSystemMetrics struct {
	HookCore
	metrics.Manager
}

func (m *actorSystemMetrics) OnActorHandleSystemMessageBefore(sender ActorRef, receiver ActorRef, message Message) {
	m.Gauge("actor_mailbox_size",
		metrics.WithTag("type", "system"),
		metrics.WithTag("ref", receiver.GetPath()),
	).Dec()
	m.Gauge("processor_message_num", metrics.WithTag("type", "system")).Inc()
}

func (m *actorSystemMetrics) OnActorHandleUserMessageBefore(sender ActorRef, receiver ActorRef, message Message) {
	m.Gauge("actor_mailbox_size",
		metrics.WithTag("type", "user"),
		metrics.WithTag("ref", receiver.GetPath()),
	).Dec()
	m.Gauge("processor_message_num", metrics.WithTag("type", "user")).Inc()
}

func (m *actorSystemMetrics) OnActorMailboxPushUserMessageBefore(ref ActorRef, message Message) {
	m.Gauge("actor_mailbox_size",
		metrics.WithTag("type", "user"),
		metrics.WithTag("ref", ref.GetPath()),
	).Inc()
}

func (m *actorSystemMetrics) OnActorMailboxPushSystemMessageBefore(ref ActorRef, message Message) {
	m.Gauge("actor_mailbox_size",
		metrics.WithTag("type", "system"),
		metrics.WithTag("ref", ref.GetPath()),
	).Inc()
}

func (m *actorSystemMetrics) OnActorLaunch(ctx ActorContext) {
	// 当前存活的 Actor 数量
	m.Gauge("alive_actor_num").Inc()
}

func (m *actorSystemMetrics) OnActorKill(ctx ActorContext, message *OnKill) {
	// 当前正在停止中的 Actor 数量
	m.Gauge("stopping_actor_num").Inc()
}

func (m *actorSystemMetrics) OnActorKilled(message *OnKilled) {
	// 当前存活的 Actor 数量
	m.Gauge("alive_actor_num").Dec()
	m.Gauge("stopping_actor_num").Dec()
}

func (m *actorSystemMetrics) hooks() []Hook {
	return []Hook{m}
}
