package metrics

import (
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/metrics"
	"github.com/kercylan98/vivid/pkg/ves"
)

var (
	_ vivid.PrelaunchActor = (*Actor)(nil)
)

// NewActor 创建一个新的指标收集 Actor。
func NewActor(updatedNotify time.Duration) *Actor {
	return &Actor{
		updatedNotify:   updatedNotify,
		lastUpdatedTime: time.Now(),
	}
}

type Actor struct {
	updatedNotify   time.Duration // 指标更新通知间隔
	lastUpdatedTime time.Time     // 上次更新时间
}

func (a *Actor) OnPrelaunch(ctx vivid.PrelaunchContext) error {
	eventStream := ctx.EventStream()
	eventStream.Subscribe(ctx, ves.ActorSpawnedEvent{})
	eventStream.Subscribe(ctx, ves.ActorLaunchedEvent{})
	eventStream.Subscribe(ctx, ves.ActorKilledEvent{})
	eventStream.Subscribe(ctx, ves.ActorRestartingEvent{})
	eventStream.Subscribe(ctx, ves.ActorRestartedEvent{})
	eventStream.Subscribe(ctx, ves.ActorFailedEvent{})
	eventStream.Subscribe(ctx, ves.ActorWatchedEvent{})
	eventStream.Subscribe(ctx, ves.ActorUnwatchedEvent{})
	eventStream.Subscribe(ctx, ves.ActorMailboxPausedEvent{})
	eventStream.Subscribe(ctx, ves.ActorMailboxResumedEvent{})
	eventStream.Subscribe(ctx, ves.DeathLetterEvent{})
	eventStream.Subscribe(ctx, ves.RemotingInboundConnectionEstablishedEvent{})
	eventStream.Subscribe(ctx, ves.RemotingOutboundConnectionEstablishedEvent{})
	eventStream.Subscribe(ctx, ves.RemotingConnectionFailedEvent{})
	eventStream.Subscribe(ctx, ves.RemotingConnectionClosedEvent{})
	eventStream.Subscribe(ctx, ves.RemotingEnvelopSendFailedEvent{})
	return nil
}

func (a *Actor) OnReceive(ctx vivid.ActorContext) {
	switch msg := ctx.Message().(type) {
	case *vivid.OnLaunch:
	case vivid.StreamEvent:
		a.onStreamEvent(ctx, msg)
	}
}

func (a *Actor) onStreamEvent(ctx vivid.ActorContext, event vivid.StreamEvent) {
	m := ctx.Metrics()
	m.Counter(metrics.StreamEventsTotalCounter).Inc()
	switch e := event.(type) {
	case ves.ActorSpawnedEvent:
		a.onActorSpawned(ctx, m, e)
	case ves.ActorLaunchedEvent:
		a.onActorLaunched(ctx, m, e)
	case ves.ActorKilledEvent:
		a.onActorKilled(ctx, m, e)
	case ves.ActorRestartingEvent:
		a.onActorRestarting(ctx, m, e)
	case ves.ActorRestartedEvent:
		a.onActorRestarted(ctx, m, e)
	case ves.ActorFailedEvent:
		a.onActorFailed(ctx, m, e)
	case ves.ActorWatchedEvent:
		a.onActorWatched(ctx, m, e)
	case ves.ActorUnwatchedEvent:
		a.onActorUnwatched(ctx, m, e)
	case ves.ActorMailboxPausedEvent:
		a.onMailboxPaused(ctx, m, e)
	case ves.ActorMailboxResumedEvent:
		a.onMailboxResumed(ctx, m, e)
	case ves.DeathLetterEvent:
		a.onDeathLetter(ctx, m, e)
	case ves.RemotingInboundConnectionEstablishedEvent:
		a.onRemotingInboundConnectionEstablished(ctx, m, e)
	case ves.RemotingOutboundConnectionEstablishedEvent:
		a.onRemotingOutboundConnectionEstablished(ctx, m, e)
	case ves.RemotingConnectionFailedEvent:
		a.onRemotingConnectionFailed(ctx, m, e)
	case ves.RemotingConnectionClosedEvent:
		a.onRemotingConnectionClosed(ctx, m, e)
	case ves.RemotingEnvelopSendFailedEvent:
		a.onRemotingEnvelopSendFailed(ctx, m, e)
	}
	a.onUpdatedNotify(ctx)
}

func (a *Actor) onUpdatedNotify(ctx vivid.ActorContext) {
	if a.updatedNotify <= -1 {
		return
	}
	if a.updatedNotify > 0 && time.Since(a.lastUpdatedTime) < a.updatedNotify {
		return
	}
	a.lastUpdatedTime = time.Now()
	ctx.EventStream().Publish(ctx, ctx.Metrics().Snapshot())
}

func (a *Actor) onActorSpawned(_ vivid.ActorContext, m metrics.Metrics, _ ves.ActorSpawnedEvent) {
	// 更新指标
	m.Counter(metrics.SpawnedActorTotalCounter).Inc()
	m.Gauge(metrics.AliveActorCountGauge).Inc()
}

func (a *Actor) onActorLaunched(_ vivid.ActorContext, m metrics.Metrics, event ves.ActorLaunchedEvent) {
	m.Histogram(metrics.ActorLaunchDurationHistogram).Observe(event.Duration.Seconds())
}

func (a *Actor) onActorKilled(_ vivid.ActorContext, m metrics.Metrics, event ves.ActorKilledEvent) {
	m.Histogram(metrics.ActorLifetimeHistogram).Observe(event.Duration.Seconds())
	m.Counter(metrics.KilledActorTotalCounter).Inc()
	m.Gauge(metrics.AliveActorCountGauge).Dec()
}

func (a *Actor) onActorRestarting(_ vivid.ActorContext, m metrics.Metrics, _ ves.ActorRestartingEvent) {
	m.Counter(metrics.RestartedActorTotalCounter).Inc()
}

func (a *Actor) onActorRestarted(_ vivid.ActorContext, m metrics.Metrics, _ ves.ActorRestartedEvent) {
	m.Counter(metrics.ActorRestartSucceededTotalCounter).Inc()
}

func (a *Actor) onActorFailed(_ vivid.ActorContext, m metrics.Metrics, _ ves.ActorFailedEvent) {
	m.Counter(metrics.ActorFailedTotalCounter).Inc()
}

func (a *Actor) onActorWatched(_ vivid.ActorContext, m metrics.Metrics, _ ves.ActorWatchedEvent) {
	m.Counter(metrics.ActorWatchTotalCounter).Inc()
	m.Gauge(metrics.ActorWatchCountGauge).Inc()
}

func (a *Actor) onActorUnwatched(_ vivid.ActorContext, m metrics.Metrics, _ ves.ActorUnwatchedEvent) {
	m.Counter(metrics.ActorUnwatchTotalCounter).Inc()
	m.Gauge(metrics.ActorWatchCountGauge).Dec()
}

func (a *Actor) onMailboxPaused(_ vivid.ActorContext, m metrics.Metrics, _ ves.ActorMailboxPausedEvent) {
	m.Counter(metrics.MailboxPausedTotalCounter).Inc()
	m.Gauge(metrics.MailboxPausedCountGauge).Inc()
}

func (a *Actor) onMailboxResumed(_ vivid.ActorContext, m metrics.Metrics, _ ves.ActorMailboxResumedEvent) {
	m.Counter(metrics.MailboxResumedTotalCounter).Inc()
	m.Gauge(metrics.MailboxPausedCountGauge).Dec()
}

func (a *Actor) onDeathLetter(_ vivid.ActorContext, m metrics.Metrics, _ ves.DeathLetterEvent) {
	// 更新指标
	m.Counter(metrics.DeathLetterTotalCounter).Inc()
}

func (a *Actor) onRemotingInboundConnectionEstablished(_ vivid.ActorContext, m metrics.Metrics, _ ves.RemotingInboundConnectionEstablishedEvent) {
	m.Counter(metrics.RemotingInboundConnectionsTotalCounter).Inc()
}

func (a *Actor) onRemotingOutboundConnectionEstablished(_ vivid.ActorContext, m metrics.Metrics, _ ves.RemotingOutboundConnectionEstablishedEvent) {
	m.Counter(metrics.RemotingOutboundConnectionsTotalCounter).Inc()
}

func (a *Actor) onRemotingConnectionFailed(_ vivid.ActorContext, m metrics.Metrics, _ ves.RemotingConnectionFailedEvent) {
	m.Counter(metrics.RemotingConnectionFailedTotalCounter).Inc()
}

func (a *Actor) onRemotingConnectionClosed(_ vivid.ActorContext, m metrics.Metrics, _ ves.RemotingConnectionClosedEvent) {
	m.Counter(metrics.RemotingConnectionClosedTotalCounter).Inc()
}

func (a *Actor) onRemotingEnvelopSendFailed(_ vivid.ActorContext, m metrics.Metrics, _ ves.RemotingEnvelopSendFailedEvent) {
	m.Counter(metrics.RemotingEnvelopSendFailedTotalCounter).Inc()
}
