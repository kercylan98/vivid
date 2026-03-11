// Package gossip 基于 Ping/Pong 的集群成员发现与视图同步。
package gossip

import (
	"fmt"
	"slices"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/internal/gossip/gossipmessages"
	"github.com/kercylan98/vivid/internal/utils"
	"github.com/kercylan98/vivid/pkg/log"
)

const (
	// GossipInterval 周期 gossip 的间隔，用于 Up 状态下持续与部分节点交换视图。
	GossipInterval = time.Second / 5
	// GossipPeersLimit 单轮 gossip 最多选择的 peer 数量，用于控制扇出与负载。
	GossipPeersLimit = 5
)

var (
	_ vivid.Actor          = (*Actor)(nil)
	_ vivid.PrelaunchActor = (*Actor)(nil)
)

// New 构造 gossip Actor。seeds 为可选种子节点引用，空时以单节点身份直接进入 Up；非空时进入 Joining 并向 seeds 发起加入。
// logger 用于 ClusterView 内部成员变更等调试日志。
func New(logger log.Logger, seeds ...vivid.ActorRef) *Actor {
	return &Actor{
		seeds:   slices.DeleteFunc(seeds, func(seed vivid.ActorRef) bool { return seed == nil }),
		view:    NewClusterView(logger),
		backoff: utils.NewExponentialBackoffWithDefault(time.Second, time.Minute),
	}
}

// Actor 实现基于 gossip 的集群发现与视图同步：维护本节点 Information、集群视图（成员列表+版本向量），
// 通过状态机事件驱动生命周期（StatusNone -> Joining -> Up），处理 Ping/Pong 与周期 gossip。
type Actor struct {
	// seeds 加入集群时使用的种子节点 ActorRef 列表，仅在 Joining 阶段使用。
	seeds []vivid.ActorRef
	// info 本节点的端点信息（Ref、Status、LastSeen），状态迁移与 Ping/Pong 时写回视图。
	info *endpoint.Information
	// view 本节点维护的集群视图：成员列表 + 版本向量，用于因果合并与 peer 选择。
	view *ClusterView
	// backoff Joining 阶段向种子发 Ping 失败时的退避策略，用于重试间隔。
	backoff *utils.ExponentialBackoff
}

// OnPrelaunch 在 Actor 启动前执行：创建本节点 Information 并加入本地视图的成员列表。
func (a *Actor) OnPrelaunch(ctx vivid.PrelaunchContext) error {
	a.info = endpoint.NewInformation(ctx.Ref())
	return a.view.Members().Add(a.info)
}

// OnReceive 消息入口：分发 OnLaunch、Ping、Pong 及调度回调（func(vivid.ActorContext)）。
func (a *Actor) OnReceive(ctx vivid.ActorContext) {
	switch m := ctx.Message().(type) {
	case *vivid.OnLaunch:
		a.onLaunch(ctx)
	case *gossipmessages.Ping:
		a.handlePing(ctx, m)
	case *gossipmessages.Pong:
		a.handlePong(ctx, m)
	case func(vivid.ActorContext):
		m(ctx)
	}
}

// onLaunch 根据是否有种子决定初始状态：无种子发 EventBootstrap 直接 Up，有种子发 EventJoinRequested 进入 Joining。
func (a *Actor) onLaunch(ctx vivid.ActorContext) {
	if len(a.seeds) == 0 {
		SendEvent(ctx, a, EventBootstrap)
		return
	}
	SendEvent(ctx, a, EventJoinRequested)
}

// handlePing 处理收到的 Ping：将发送方加入视图（新成员则合并其版本并触发一次 gossip）、若对方视图更新则合并，最后回 Pong。
func (a *Actor) handlePing(ctx vivid.ActorContext, ping *gossipmessages.Ping) {
	if a.view.Members().Add(ping.Info) == nil {
		a.view.Version().Merge(ping.Version)
		ctx.Tell(ctx.Ref(), func(ctx vivid.ActorContext) { a.SendPingTo(ctx) })
	}
	if a.view.Version().IsBefore(ping.Version) {
		a.view.Members().Merge(ping.MemberList)
		a.view.Version().Merge(ping.Version)
	}
	ctx.Reply(gossipmessages.NewPong(a.info, a.view.Members(), a.view.Version()))
}

// handlePong 处理 Ask 得到的 Pong：若对方版本更新则合并其成员与版本；若本节点当前为 Joining 则发 EventJoined 迁到 Up。
func (a *Actor) handlePong(ctx vivid.ActorContext, pong *gossipmessages.Pong) {
	if a.view.Version().IsBefore(pong.Version) {
		a.view.Members().Merge(pong.MemberList)
		a.view.Version().Merge(pong.Version)
	}
	if a.info.Status == endpoint.StatusJoining {
		ctx.Logger().Debug("joined cluster")
		SendEvent(ctx, a, EventJoined)
	}
}

// StartJoining 向种子节点发送 Ping 拉取视图；若仍处于 Joining 则按 backoff 安排一次重试。
func (a *Actor) StartJoining(ctx vivid.ActorContext) {
	ctx.Logger().Debug("joining cluster", log.String("seeds", vivid.ActorRefs(a.seeds).String()))
	a.SendPingTo(ctx, a.seeds...)
	if a.info.Status == endpoint.StatusJoining {
		ctx.Scheduler().Once(ctx.Ref(), a.backoff.Next(), func(ctx vivid.ActorContext) { a.StartJoining(ctx) })
	}
}

// StartGossipLoop 注册周期任务，按 GossipInterval 向视图中部分 Up 节点发起 gossip。
func (a *Actor) StartGossipLoop(ctx vivid.ActorContext) {
	ctx.Scheduler().Loop(ctx.Ref(), GossipInterval, func(ctx vivid.ActorContext) { a.SendPingTo(ctx) })
}

// SendPingTo 向 targets 发 Ping 并同步处理每个 Pong；targets 为空时从视图中取最多 GossipPeersLimit 个 Up 节点作为目标。
func (a *Actor) SendPingTo(ctx vivid.ActorContext, targets ...vivid.ActorRef) {
	var joiningMode bool
	if joiningMode = len(targets) != 0; !joiningMode {
		peers := a.view.Members().Unseens(a.info, GossipPeersLimit)
		if len(peers) == 0 {
			return
		}
		targets = make([]vivid.ActorRef, 0, len(peers))
		for _, p := range peers {
			targets = append(targets, p.ActorRef)
		}
	}
	ping := gossipmessages.NewPing(a.info, a.view.Members(), a.view.Version())

	// 非加入时转为非阻塞模式
	if joiningMode {
		for _, ref := range targets {
			result, err := ctx.Ask(ref, ping).Result()
			if err != nil {
				ctx.Logger().Error("gossip failed", log.String("ref", ref.String()), log.String("error", err.Error()))
				continue
			}
			pong, ok := result.(*gossipmessages.Pong)
			if !ok {
				ctx.Logger().Error("gossip invalid pong", log.String("ref", ref.String()), log.String("type", fmt.Sprintf("%T", result)))
				continue
			}
			a.handlePong(ctx, pong)
		}
	} else {
		for _, ref := range targets {
			ctx.Tell(ref, ping)
		}
	}

}
