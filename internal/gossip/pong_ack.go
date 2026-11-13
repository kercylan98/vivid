package gossip

import (
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/internal/gossip/gossipmessages"
)

func (a *Actor) replyPongAck(ctx vivid.ActorContext, ping *gossipmessages.Ping) {
	view := a.currentOutboundView()
	ack := &gossipmessages.PongAck{
		ClusterName: a.opts.Name, GossipType: ping.GossipType, Timestamp: time.Now(),
		Info: view.info, Revision: view.version.Get(view.info.ID()), ProbeSequence: ping.ProbeSequence,
	}
	if ping.ProbeSequence != 0 {
		ack.ProbeOriginID = ping.Info.ID()
	}
	ctx.Reply(ack)
}

// onPongAck 只接受已知实例当前状态的直接应答，不合并成员或导入版本。
func (a *Actor) onPongAck(ctx vivid.ActorContext, ack *gossipmessages.PongAck) {
	if ack == nil || ack.ClusterName != a.opts.Name || ack.Info == nil || ack.Info.ActorRef == nil ||
		a.info.Status == endpoint.StatusNone || a.info.Status == endpoint.StatusJoining {
		return
	}
	member := a.view.Members().Get(ack.Info.ID())
	if member == nil || ack.Revision == 0 || ack.Revision != a.view.Version().Get(member.ID()) ||
		member.Status != ack.Info.Status || member.DataCenter != ack.Info.DataCenter || !member.CreatedAt.Equal(ack.Info.CreatedAt) {
		return
	}
	now := time.Now()
	a.refreshReachability(ctx, now)
	a.observeResponse(ctx, ack.Info, ack.ProbeSequence, ack.ProbeOriginID, ack.Revision, now)
}
