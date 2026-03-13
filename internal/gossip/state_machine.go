package gossip

import (
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/internal/gossip/gossipmessages"
	"github.com/kercylan98/vivid/pkg/log"
)

// Event 状态机事件，用于驱动节点状态迁移（仅在有合法转换时生效）。
type Event int

const (
	EventBootstrap Event = iota + 1 // 无种子，以单节点身份直接进入 Up
	EventJoining                    // 有种子，请求加入集群，进入 Joining
	EventUp                         // 已收到集群 Pong，从 Joining 迁到 Up
	EventLeaving                    // 请求离开集群，进入 Leaving
	EventExiting                    // 离开集群完成，进入 Exiting
	EventRemoved                    // 已从集群中移除，已完全离开集群
)

// transition 描述一次合法的状态迁移的结果。
type transition struct {
	to                endpoint.Status // 目标状态
	immediatelyGossip bool            // 是否立即 gossip
}

// transitions 状态机转换表：(当前状态, 事件) -> (目标状态, 是否立即 gossip)。
var transitions = map[endpoint.Status]map[Event]transition{
	endpoint.StatusNone: {
		EventJoining:   {endpoint.StatusJoining, false},
		EventBootstrap: {endpoint.StatusUp, true},
	},
	endpoint.StatusJoining: {
		EventUp: {endpoint.StatusUp, true},
	},
	endpoint.StatusUp: {
		EventLeaving: {endpoint.StatusLeaving, true},
	},
	endpoint.StatusLeaving: {
		EventExiting: {endpoint.StatusExiting, true},
	},
	endpoint.StatusExiting: {
		EventRemoved: {endpoint.StatusRemoved, true},
	},
}

// changeStatus 根据当前状态与事件执行迁移：若 (a.info.Status, e) 无合法转换则打日志并返回 false；
// 否则更新状态、递增本节点版本并写回视图、执行目标状态的入口动作（StartJoining / StartGossipLoop），
// 若转换配置了立即 gossip 则调用 SendPingTo。
func changeStatus(ctx vivid.ActorContext, a *Actor, e Event) bool {
	t, ok := transitions[a.info.Status][e]
	if !ok {
		ctx.Logger().Error("invalid transition",
			log.String("state", a.info.Status.String()),
			log.Int("event", int(e)))
		return false
	}

	a.info.Status = t.to
	a.view.Version().Increment(a.info.ID())
	a.view.Members().Upsert(a.info)

	ctx.Tell(ctx.Ref(), a.info.Status)

	if t.immediatelyGossip {
		ctx.Tell(ctx.Ref(), gossipmessages.NewSpreadGossip())
	}
	return true
}
