package gossip

import (
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/internal/gossip/versionvector"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type viewStableContext struct {
	joinContext
	message       vivid.Message
	notifications []*viewStable
}

func (c *viewStableContext) Message() vivid.Message { return c.message }

func (c *viewStableContext) Tell(_ vivid.ActorRef, message vivid.Message) {
	if notification, ok := message.(*viewStable); ok {
		c.notifications = append(c.notifications, notification)
	}
}

func (c *viewStableContext) deliver(a *Actor, notification *viewStable) {
	c.message = notification
	a.OnReceive(c)
}

func newViewStableActor(status endpoint.Status) (*Actor, *viewStableContext) {
	local := endpoint.NewInformation(&tombstoneRef{address: "local:10001"}, time.Unix(1, 0))
	local.Status = status
	a := New(vivid.NewActorSystemClusterOptions(vivid.WithActorSystemClusterName("vivid-test")))
	a.info, a.view = local, NewClusterView(log.GetDefault())
	a.view.Members().Upsert(local)
	a.view.Version().Increment(local.ID())
	a.phaseKillCompleted = make(chan struct{})
	return a, &viewStableContext{joinContext: joinContext{tombstoneContext{ref: local.ActorRef}}}
}

func queueViewStable(t *testing.T, a *Actor, ctx *viewStableContext) *viewStable {
	t.Helper()
	before := len(ctx.notifications)
	for range 4 {
		maybeEmitViewStable(ctx, a)
	}
	require.Len(t, ctx.notifications, before+1)
	return ctx.notifications[before]
}

func TestActor_GossipViewStableStale(t *testing.T) {
	for _, name := range []string{"local_status", "member_added", "member_version", "old_instance", "superseded", "nil", "unissued"} {
		t.Run(name, func(t *testing.T) {
			a, ctx := newViewStableActor(endpoint.StatusLeaving)
			remote := endpoint.NewInformation(&tombstoneRef{address: "remote:10002"}, time.Unix(2, 0))
			remote.Status = endpoint.StatusUp
			if name == "local_status" {
				a.info.Status = endpoint.StatusUp
			}
			if name == "member_version" {
				a.view.Members().Upsert(remote)
				a.view.Version().Update(remote.ID(), 1)
			}
			notification := queueViewStable(t, a, ctx)
			switch name {
			case "local_status":
				a.onKill(ctx, &vivid.OnKill{})
			case "member_added", "member_version", "superseded":
				version := versionvector.New()
				version.Update(remote.ID(), 2)
				a.mergeMember(remote, version)
				a.publishMembersChanged(ctx)
				if name == "superseded" {
					queueViewStable(t, a, ctx)
				}
			case "old_instance":
				a.info.InstanceID = "new-local-instance"
			case "nil":
				notification = nil
			case "unissued":
				copy := *notification
				notification = &copy
			}
			ctx.deliver(a, notification)
			assert.Equal(t, endpoint.StatusLeaving, a.info.Status, "无效稳定通知不能推进退出阶段")
			assert.Empty(t, a.coordinatorNodeID, "无效通知不能执行协调者更新")
			select {
			case <-a.phaseKillCompleted:
				t.Fatal("无效通知不能完成停止")
			default:
			}
			if name == "superseded" {
				ctx.deliver(a, ctx.notifications[len(ctx.notifications)-1])
				assert.Equal(t, endpoint.StatusExiting, a.info.Status, "旧通知不能清除新通知")
			}
		})
	}
}

func TestActor_GossipViewStableShutdown(t *testing.T) {
	a, ctx := newViewStableActor(endpoint.StatusUp)
	old := queueViewStable(t, a, ctx)
	a.onKill(ctx, &vivid.OnKill{})
	ctx.deliver(a, old)
	require.Equal(t, endpoint.StatusLeaving, a.info.Status)
	for _, next := range []endpoint.Status{endpoint.StatusExiting, endpoint.StatusRemoved} {
		notification := queueViewStable(t, a, ctx)
		ctx.deliver(a, notification)
		require.Equal(t, next, a.info.Status)
		ctx.deliver(a, notification)
		require.Equal(t, next, a.info.Status, "重复通知不能再次推进状态")
		select {
		case <-a.phaseKillCompleted:
			t.Fatal("Removed 视图尚未稳定，不应提前完成停止")
		default:
		}
	}
	ctx.deliver(a, queueViewStable(t, a, ctx))
	select {
	case <-a.phaseKillCompleted:
	default:
		t.Fatal("当前 Removed 视图稳定后应完成停止")
	}
}

func TestActor_GossipViewStableOnce(t *testing.T) {
	a, ctx := newViewStableActor(endpoint.StatusUp)
	first := queueViewStable(t, a, ctx)
	ctx.deliver(a, first)
	assert.Equal(t, a.info.ID(), a.coordinatorNodeID)
	assert.Nil(t, a.pendingViewStable)
	for range 10 {
		maybeEmitViewStable(ctx, a)
		ctx.deliver(a, first)
	}
	assert.Len(t, ctx.notifications, 1, "同一视图只通知一次")
	assert.True(t, changeStatus(ctx, a, EventLeaving))
	queueViewStable(t, a, ctx)
	assert.Len(t, ctx.notifications, 2, "新视图重新观察后可以通知")
}

func TestActor_GossipViewStableReachability(t *testing.T) {
	a, ctx := newViewStableActor(endpoint.StatusLeaving)
	remote := endpoint.NewInformation(&tombstoneRef{address: "remote:10002"}, time.Unix(2, 0))
	remote.Status = endpoint.StatusUp
	a.view.Members().Upsert(remote)
	a.view.Version().Update(remote.ID(), 1)
	a.reachability = newLocalReachability()
	notification := queueViewStable(t, a, ctx)
	revision := a.viewRevision
	a.reachability.observe(remote.ID(), vivid.ClusterReachabilityUnreachable, time.Now(), time.Second)
	a.publishReachabilityChanged(ctx)
	require.Greater(t, a.viewRevision, revision)
	ctx.deliver(a, notification)
	assert.Equal(t, endpoint.StatusExiting, a.info.Status, "本地可达性变化不改变成员视图，不能使退出稳定通知失效")
}

type viewStablePrelaunchContext struct{ seedContext }

func (*viewStablePrelaunchContext) Logger() log.Logger { return log.GetDefault() }

func (*viewStablePrelaunchContext) WithPhaseKill(<-chan struct{}, time.Duration, vivid.Behavior) error {
	return nil
}

func TestActor_GossipViewStableRestart(t *testing.T) {
	a, ctx := newViewStableActor(endpoint.StatusUp)
	notification := queueViewStable(t, a, ctx)
	previousID := a.info.ID()
	prelaunch := &viewStablePrelaunchContext{seedContext{ref: a.info.ActorRef}}
	require.NoError(t, a.OnPrelaunch(prelaunch))
	require.NotEqual(t, previousID, a.info.ID())
	assert.Nil(t, a.pendingViewStable)
	assert.False(t, a.viewStableEmitted)
	assert.Zero(t, a.stableRounds)
	assert.Empty(t, a.lastViewFingerprint)
	ctx.deliver(a, notification)
	require.Equal(t, endpoint.StatusNone, a.info.Status)
	require.True(t, changeStatus(ctx, a, EventJoining))
	require.Equal(t, endpoint.StatusUp, a.info.Status)
	ctx.deliver(a, queueViewStable(t, a, ctx))
	assert.Nil(t, a.pendingViewStable, "新实例能够独立完成稳定通知")
}
