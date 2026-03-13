// Package gossip 基于 Ping/Pong 的集群成员发现与视图同步。
package gossip

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"sort"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/internal/gossip/gossipmessages"
	"github.com/kercylan98/vivid/pkg/log"
)

const (
	// singleSchedulerReference 单个调度器的引用，用于周期 gossip。
	singleSchedulerReference = "single"
)

var (
	_ vivid.Actor          = (*Actor)(nil)
	_ vivid.PrelaunchActor = (*Actor)(nil)
)

// New 构造 gossip Actor。seeds 为可选种子节点引用，空时以单节点身份直接进入 Up；非空时进入 Joining 并向 seeds 发起加入。
// logger 用于 ClusterView 内部成员变更等调试日志。
func New(logger log.Logger, options ...Option) *Actor {
	opts := NewOptions(options...)
	return &Actor{
		opts: opts,
		view: NewClusterView(logger),
	}
}

// Actor 实现基于 gossip 的集群发现与视图同步：维护本节点 Information、集群视图（成员列表+版本向量），
// 通过状态机事件驱动生命周期，处理 Ping/Pong 与周期 gossip。
type Actor struct {
	opts                 *Options              // 配置选项
	info                 *endpoint.Information // 本节点的端点信息（Ref、Status、LastSeen），状态迁移与 Ping/Pong 时写回视图。
	view                 *ClusterView          // 本节点维护的集群视图：成员列表 + 版本向量，用于因果合并与 peer 选择。
	lastViewFingerprint  string                // 收敛检测：视图指纹连续不变时投递 Converged，视图变化后重置以便再次收敛时投递。
	convergedEmitted     bool                  // 是否已经投递过 Converged 消息
	stableRounds         int                   // 连续不变的轮次
	phaseKillCompleted   chan struct{}         // 多阶段终止流程完成信号。
	coordinatorNodeID    string                // 当前的协调者节点 ID
	convergenceStartedAt time.Time             // 最近一次视图变化的时间，用于计算收敛耗时
}

// OnPrelaunch 在 Actor 启动前执行：创建本节点 Information 并加入本地视图的成员列表。
func (a *Actor) OnPrelaunch(ctx vivid.PrelaunchContext) error {
	// 打乱 seeds 顺序
	rand.Shuffle(len(a.opts.Seeds), func(i, j int) {
		a.opts.Seeds[i], a.opts.Seeds[j] = a.opts.Seeds[j], a.opts.Seeds[i]
	})

	// 注册多阶段终止以支持优雅退出
	a.phaseKillCompleted = make(chan struct{})
	if err := ctx.WithPhaseKill(a.phaseKillCompleted, a.opts.GracefulShutdownTimeout, a.OnReceive); err != nil {
		return err
	}

	// 创建本节点 Information 并加入本地视图的成员列表
	a.info = endpoint.NewInformation(ctx.Ref(), time.Now().Add(a.opts.LaunchDelay))
	if !a.view.Members().Upsert(a.info) {
		return vivid.ErrorException.WithMessage("prelaunch failed, unexpected error")
	}

	// 初始化当前的协调者节点 ID
	a.coordinatorNodeID = a.view.Members().GetCoordinatorNodeID()
	return nil
}

// OnReceive 消息入口：分发 OnLaunch、Ping、Pong 及调度回调（func(vivid.ActorContext)）。
func (a *Actor) OnReceive(ctx vivid.ActorContext) {
	switch m := ctx.Message().(type) {
	case *vivid.OnLaunch:
		a.onLaunch(ctx)
	case *gossipmessages.Ping:
		a.onPing(ctx, m)
	case *gossipmessages.Pong:
		a.onPong(ctx, m)
	case *gossipmessages.SpreadGossip:
		a.onSpreadGossip(ctx)
	case *gossipmessages.Converged:
		a.onConverged(ctx)
	case endpoint.Status:
		a.onStatusChanged(ctx, m)
	case *vivid.OnKill:
		a.onKill(ctx, m)
	}
}

func (a *Actor) onStatusChanged(ctx vivid.ActorContext, status endpoint.Status) {
	switch status {
	case endpoint.StatusJoining:
		a.onJoining(ctx)
	case endpoint.StatusUp:
		a.onUp(ctx)
	case endpoint.StatusLeaving:
		a.onLeaving(ctx)
	case endpoint.StatusExiting:
		a.onExiting(ctx)
	case endpoint.StatusRemoved:
		a.onRemoved(ctx)
	default:
	}
}

// onLaunch 根据是否有种子决定初始状态：无种子发 EventBootstrap 直接 Up，有种子发 EventJoinRequested 进入 Joining；并注册多阶段终止以支持优雅退出。
func (a *Actor) onLaunch(ctx vivid.ActorContext) {
	// 随机延迟启动，避免所有节点同时启动，导致集群瞬间压力过大
	time.Sleep(a.opts.LaunchDelay)

	// 固定种子节点顺序
	sort.Slice(a.opts.Seeds, func(i, j int) bool {
		aAddr := a.opts.Seeds[i].GetAddress()
		bAddr := a.opts.Seeds[j].GetAddress()
		return aAddr < bAddr
	})

	// 尝试加入
	changeStatus(ctx, a, EventJoining)
}

func (a *Actor) onJoining(ctx vivid.ActorContext) {
	if a.info.Status != endpoint.StatusJoining {
		return // 如果当前状态不是 Joining，则不进行重试，虽然只是 gossip 同步，但是会增加不必要的开销
	}

	// 检查种子节点是否包含自己
	seedsSelfIndex := slices.IndexFunc(a.opts.Seeds, func(seed vivid.ActorRef) bool { return seed.Equals(ctx.Ref()) })

	// 尝试加入所有种子节点
	ping := gossipmessages.NewPing(a.info, a.view.Members(), a.view.Version())
	for i, seed := range a.opts.Seeds {
		// 如果种子节点是自己，则跳过
		if i == seedsSelfIndex {
			continue
		}
		result, err := ctx.Ask(seed, ping, a.opts.JoinTimeout).Result()
		if err != nil {
			ctx.Logger().Error("failed to join cluster, try next seed", log.String("seed", seed.String()), log.Any("error", err))
			continue
		}
		pong, ok := result.(*gossipmessages.Pong)
		if !ok {
			ctx.Logger().Error("failed to join cluster, try next seed", log.String("seed", seed.String()), log.Any("result", result),
				log.Any("error", vivid.ErrorFutureMessageTypeMismatch.WithMessage(fmt.Sprintf("expected *gossipmessages.Pong, got %T", result))))
			continue
		}
		a.onPong(ctx, pong)
		changeStatus(ctx, a, EventUp)
		ctx.Logger().Debug("joined cluster")
		return
	}

	// 所有种子节点都失败了，自举为孤岛集群
	changeStatus(ctx, a, EventUp)
}

func (a *Actor) onUp(ctx vivid.ActorContext) {
	// 注册周期任务，按 [GossipInterval] 向视图中节点发起 gossip。
	spreadGossipMessage := gossipmessages.NewSpreadGossip()
	if err := ctx.Scheduler().Loop(ctx.Ref(), a.opts.GossipInterval, spreadGossipMessage, vivid.WithSchedulerReference(singleSchedulerReference)); err != nil {
		// 报告故障以可以支持监管者重启
		ctx.Failed(vivid.ErrorGossipScheduleFailed.With(err).WithMessage("failed to schedule spread gossip"))
	}
}
func (a *Actor) onConverged(ctx vivid.ActorContext) {
	a.view.cleanRemovedMembers(a.info)

	// 计算新的协调者节点 ID
	a.coordinatorNodeID = a.view.Members().GetCoordinatorNodeID()

	ctx.Logger().Debug("cluster converged",
		log.String("coordinator", a.coordinatorNodeID),
		log.String("view", a.view.Fingerprint()),
		log.Duration("convergence_duration", time.Since(a.convergenceStartedAt)),
	)

	for _, member := range a.view.Members().List() {
		ctx.Logger().Info("cluster_view",
			log.String("id", member.ID()),
			log.String("status", member.Status.String()),
			log.Int64("created_at", member.CreatedAt.UnixNano()),
		)
	}

	// 自身状态迁移
	switch a.info.Status {
	case endpoint.StatusLeaving:
		changeStatus(ctx, a, EventExiting)
	case endpoint.StatusExiting:
		changeStatus(ctx, a, EventRemoved)
	default:
	}
}

// onPing 处理收到的 Ping 消息：
//   - 仅当对方在该成员 key 上的版本不旧于本地时，才接受发送方的自描述（避免迟到 Ping 用旧状态覆盖已更新的视图）。
//   - 若对方整体版本更新（IsBefore），则合并其整表成员与版本。
//   - 最后回复 Pong，带上当前节点信息、成员列表和版本向量。
func (a *Actor) onPing(ctx vivid.ActorContext, ping *gossipmessages.Ping) {
	id := ping.Info.ID()
	if ping.Version.Get(id) >= a.view.Version().Get(id) {
		isNew := a.view.Members().Upsert(ping.Info)
		a.view.Version().Merge(ping.Version)
		if isNew {
			ctx.Tell(ctx.Ref(), gossipmessages.NewSpreadGossip())
		}
	}

	// 如果对方版本更新，则合并其成员与版本
	if a.view.Version().IsBefore(ping.Version) {
		a.view.Members().Merge(ping.MemberList)
		a.view.Version().Merge(ping.Version)
	}
	// 合并后写回本节点信息，避免被对方视图中本节点的旧状态（如 JOINING）覆盖已迁移的 UP
	a.view.Members().Upsert(a.info)
	maybeEmitConverged(ctx, a)

	ctx.Reply(gossipmessages.NewPong(a.info, a.view.Members(), a.view.Version()))
}

// onPong 处理 Ask 得到的 Pong：若对方版本更新则合并其成员与版本；若本节点当前为 Joining 则发 EventJoined 迁到 Up。
func (a *Actor) onPong(ctx vivid.ActorContext, pong *gossipmessages.Pong) {
	if a.view.Version().IsBefore(pong.Version) {
		a.view.Members().Merge(pong.MemberList)
		a.view.Version().Merge(pong.Version)
	}
	// 合并后写回本节点信息，避免被对方视图中本节点的旧状态（如 JOINING）覆盖已迁移的 UP
	a.view.Members().Upsert(a.info)
	maybeEmitConverged(ctx, a)
}

func (a *Actor) onKill(ctx vivid.ActorContext, _ *vivid.OnKill) {
	// 处理本节点告知即将离开集群
	changeStatus(ctx, a, EventLeaving)
}

func (a *Actor) onLeaving(ctx vivid.ActorContext) {
	// 处理即将离开集群
	// 目前无需任何处理，gossip 的触发已经在状态变更时被处理了
	ctx.Logger().Debug("leaving cluster")
}

func (a *Actor) onExiting(ctx vivid.ActorContext) {
	// 处理即将离开集群
	// 目前无需任何处理，gossip 的触发已经在状态变更时被处理了
	ctx.Logger().Debug("exiting cluster")
}

func (a *Actor) onRemoved(ctx vivid.ActorContext) {
	// 已离开集群，结束多阶段流程
	ctx.Logger().Debug("removed from cluster")
	close(a.phaseKillCompleted)
}

// onSpreadGossip 向 targets 发 Ping 并同步处理每个 Pong；targets 为空时从视图中取最多 GossipPeersLimit 个 Up 节点作为目标。
func (a *Actor) onSpreadGossip(ctx vivid.ActorContext) {
	peers := a.view.Members().Unseens(a.info, a.opts.Seeds, a.opts.GossipPeersCount)
	if len(peers) == 0 {
		maybeEmitConverged(ctx, a)
		return
	}

	ping := gossipmessages.NewPing(a.info, a.view.Members(), a.view.Version())

	for _, peer := range peers {
		ctx.Tell(peer, ping)
	}
}
