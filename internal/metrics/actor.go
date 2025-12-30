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
		actorSpawnedTimes: make(map[string]time.Time),
		actorLaunchTimes:  make(map[string]time.Time),
		actorRestartTimes: make(map[string]time.Time),
		mailboxPauseTimes: make(map[string]time.Time),
		updatedNotify:     updatedNotify,
		lastUpdatedTime:   time.Now(),
	}
}

type Actor struct {
	updatedNotify     time.Duration        // 指标更新通知间隔
	lastUpdatedTime   time.Time            // 上次更新时间
	actorSpawnedTimes map[string]time.Time // Actor 创建时间，用于计算启动耗时
	actorLaunchTimes  map[string]time.Time // Actor 启动时间，用于计算生命周期
	actorRestartTimes map[string]time.Time // Actor 重启开始时间，用于计算重启耗时
	mailboxPauseTimes map[string]time.Time // 邮箱暂停时间，用于计算暂停时长
	metrics           metrics.Metrics      // 指标收集器
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
	return nil
}

func (a *Actor) OnReceive(ctx vivid.ActorContext) {
	switch msg := ctx.Message().(type) {
	case *vivid.OnLaunch:
		a.metrics = ctx.Metrics()
	case vivid.StreamEvent:
		a.onStreamEvent(ctx, msg)
	}
}

func (a *Actor) onStreamEvent(ctx vivid.ActorContext, event vivid.StreamEvent) {
	switch e := event.(type) {
	case ves.ActorSpawnedEvent:
		a.onActorSpawned(e)
	case ves.ActorLaunchedEvent:
		a.onActorLaunched(e)
	case ves.ActorKilledEvent:
		a.onActorKilled(e)
	case ves.ActorRestartingEvent:
		a.onActorRestarting(e)
	case ves.ActorRestartedEvent:
		a.onActorRestarted(e)
	case ves.ActorFailedEvent:
		a.onActorFailed(e)
	case ves.ActorWatchedEvent:
		a.onActorWatched(e)
	case ves.ActorUnwatchedEvent:
		a.onActorUnwatched(e)
	case ves.ActorMailboxPausedEvent:
		a.onMailboxPaused(e)
	case ves.ActorMailboxResumedEvent:
		a.onMailboxResumed(e)
	case ves.DeathLetterEvent:
		a.onDeathLetter(e)
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

func (a *Actor) onActorSpawned(event ves.ActorSpawnedEvent) {
	path := event.ActorRef.GetPath()
	now := time.Now()
	a.actorSpawnedTimes[path] = now

	// 更新指标
	a.metrics.Counter("vivid_actor_spawned_total").Inc()
	a.metrics.Gauge("vivid_actor_count").Inc()
	a.metrics.Counter("vivid_actor_spawned_total_by_type").Add(1) // 可以按类型进一步细分
}

func (a *Actor) onActorLaunched(event ves.ActorLaunchedEvent) {
	path := event.ActorRef.GetPath()
	now := time.Now()
	a.actorLaunchTimes[path] = now

	// 计算启动耗时
	if spawnedTime, ok := a.actorSpawnedTimes[path]; ok {
		duration := now.Sub(spawnedTime).Seconds()
		a.metrics.Histogram("vivid_actor_launch_duration_seconds").Observe(duration)
		delete(a.actorSpawnedTimes, path)
	}
}

func (a *Actor) onActorKilled(event ves.ActorKilledEvent) {
	path := event.ActorRef.GetPath()

	// 计算生命周期时长
	if launchTime, ok := a.actorLaunchTimes[path]; ok {
		duration := time.Since(launchTime).Seconds()
		a.metrics.Histogram("vivid_actor_lifetime_seconds").Observe(duration)
		delete(a.actorLaunchTimes, path)
	}

	// 清理相关时间记录
	delete(a.actorSpawnedTimes, path)
	delete(a.actorRestartTimes, path)
	delete(a.mailboxPauseTimes, path)

	// 更新指标
	a.metrics.Counter("vivid_actor_killed_total").Inc()
	a.metrics.Gauge("vivid_actor_count").Dec()
}

func (a *Actor) onActorRestarting(event ves.ActorRestartingEvent) {
	path := event.ActorRef.GetPath()
	a.actorRestartTimes[path] = time.Now()

	// 更新指标
	a.metrics.Counter("vivid_actor_restart_total").Inc()
	a.metrics.Counter("vivid_actor_restart_total_by_type").Add(1)
}

func (a *Actor) onActorRestarted(event ves.ActorRestartedEvent) {
	path := event.ActorRef.GetPath()

	// 计算重启耗时
	if restartTime, ok := a.actorRestartTimes[path]; ok {
		duration := time.Since(restartTime).Seconds()
		a.metrics.Histogram("vivid_actor_restart_duration_seconds").Observe(duration)
		delete(a.actorRestartTimes, path)
	}

	// 更新指标
	a.metrics.Counter("vivid_actor_restart_success_total").Inc()
}

func (a *Actor) onActorFailed(_ ves.ActorFailedEvent) {
	// 更新指标
	a.metrics.Counter("vivid_actor_failed_total").Inc()
	a.metrics.Counter("vivid_actor_failed_total_by_type").Add(1)
}

func (a *Actor) onActorWatched(_ ves.ActorWatchedEvent) {
	// 更新指标
	a.metrics.Counter("vivid_actor_watch_total").Inc()
	a.metrics.Gauge("vivid_actor_watch_count").Inc()
}

func (a *Actor) onActorUnwatched(_ ves.ActorUnwatchedEvent) {
	// 更新指标
	a.metrics.Counter("vivid_actor_unwatch_total").Inc()
	a.metrics.Gauge("vivid_actor_watch_count").Dec()
}

func (a *Actor) onMailboxPaused(event ves.ActorMailboxPausedEvent) {
	path := event.ActorRef.GetPath()
	a.mailboxPauseTimes[path] = time.Now()

	// 更新指标
	a.metrics.Counter("vivid_mailbox_paused_total").Inc()
	a.metrics.Gauge("vivid_mailbox_paused_count").Inc()
}

func (a *Actor) onMailboxResumed(event ves.ActorMailboxResumedEvent) {
	path := event.ActorRef.GetPath()

	// 计算暂停时长
	if pauseTime, ok := a.mailboxPauseTimes[path]; ok {
		duration := time.Since(pauseTime).Seconds()
		a.metrics.Histogram("vivid_mailbox_paused_duration_seconds").Observe(duration)
		delete(a.mailboxPauseTimes, path)
	}

	// 更新指标
	a.metrics.Counter("vivid_mailbox_resumed_total").Inc()
	a.metrics.Gauge("vivid_mailbox_paused_count").Dec()
}

func (a *Actor) onDeathLetter(_ ves.DeathLetterEvent) {
	// 更新指标
	a.metrics.Counter("vivid_death_letter_total").Inc()
}

// GetMetrics 返回指标收集器实例。
func (a *Actor) GetMetrics() metrics.Metrics {
	return a.metrics
}
