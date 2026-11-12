package gossip

import (
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/internal/gossip/gossipmessages"
)

// SnapshotActor 仅用于测试，在 Actor 消息线程中采集视图，避免测试并发读取可变成员列表。
type SnapshotActor struct {
	*Actor
	pingCount map[string]uint64
}

type TestSnapshotRequest struct{}

type SnapshotResponse struct {
	Snapshot  vivid.ClusterSnapshot
	ID        string
	Status    endpoint.Status
	View      *ClusterView
	PingCount map[string]uint64
}

func NewSnapshotActor(options *vivid.ActorSystemClusterOptions) *SnapshotActor {
	return &SnapshotActor{Actor: New(options), pingCount: make(map[string]uint64)}
}

func (a *SnapshotActor) OnReceive(ctx vivid.ActorContext) {
	switch message := ctx.Message().(type) {
	case TestSnapshotRequest:
		a.refreshReachability(ctx, time.Now())
		counts := make(map[string]uint64, len(a.pingCount))
		for key, count := range a.pingCount {
			counts[key] = count
		}
		ctx.Reply(SnapshotResponse{
			Snapshot:  a.currentSnapshot(),
			ID:        a.info.ID(),
			Status:    a.info.Status,
			View:      &ClusterView{members: a.view.Members().Clone(), version: a.view.Version().Clone()},
			PingCount: counts,
		})
	case *gossipmessages.Ping:
		a.pingCount[message.Info.ActorRef.GetAddress()]++
		a.Actor.OnReceive(ctx)
	default:
		a.Actor.OnReceive(ctx)
	}
}
