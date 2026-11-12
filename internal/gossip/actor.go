// Package gossip 基于 Ping/Pong 的集群成员发现与视图同步。
package gossip

import (
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/internal/gossip/gossipmessages"
	"github.com/kercylan98/vivid/internal/gossip/versionvector"
	"github.com/kercylan98/vivid/pkg/log"
)

const (
	// singleSchedulerReference 单个调度器的引用，用于周期 gossip。
	singleSchedulerReference = "single"
	gossipActorName          = "@gossip"
)

var (
	_ vivid.Actor            = (*Actor)(nil)
	_ vivid.FixedOptionActor = (*Actor)(nil)
	_ vivid.PrelaunchActor   = (*Actor)(nil)
)

// New 构造 gossip Actor。seeds 为可选种子节点引用，空时以单节点身份直接进入 Up；非空时进入 Joining 并向 seeds 发起加入。
func New(options *vivid.ActorSystemClusterOptions) *Actor {
	return &Actor{opts: options, readiness: newReadiness()}
}

// Actor 实现基于 gossip 的集群发现与视图同步：维护本节点 Information、集群视图（成员列表+版本向量），
// 通过状态机事件驱动生命周期，处理 Ping/Pong 与周期 gossip。
type Actor struct {
	opts                 *vivid.ActorSystemClusterOptions // 配置选项
	seeds                []vivid.ActorRef                 // 种子节点引用列表
	info                 *endpoint.Information            // 本节点的实例信息（ID、Ref、Status），状态迁移与 Ping/Pong 时写回视图。
	view                 *ClusterView                     // 本节点维护的集群视图：成员列表 + 版本向量，用于因果合并与 peer 选择。
	lastViewFingerprint  string                           // 收敛检测：视图指纹连续不变时投递 Converged，视图变化后重置以便再次收敛时投递。
	convergedEmitted     bool                             // 是否已经投递过 Converged 消息
	stableRounds         int                              // 连续不变的轮次
	phaseKillCompleted   chan struct{}                    // 多阶段终止流程完成信号。
	coordinatorNodeID    string                           // 当前的协调者节点 ID
	convergenceStartedAt time.Time                        // 最近一次视图变化的时间，用于计算收敛耗时
	viewRevision         uint64                           // 本地观察序号，与本实例 ID 一起标识快照
	snapshot             *clusterSnapshot                 // 当前视图的只读快照
	readiness            *Readiness                       // 线程安全的初始加入结果
	joinRequest          vivid.Future[vivid.Message]      // 当前种子的加入请求
	joinIndex            int                              // 当前尝试的去重种子下标
	joinAttempt          int                              // 当前种子的请求次数
	joinDeadline         time.Time                        // 当前种子的加入总截止时间
	joinError            error                            // 最近一次加入失败
	stopping             bool                             // 已开始停止，不再接受迟到的加入结果
	membersChanged       bool                             // 本次合并是否接收了成员更新
	reachability         *localReachability               // 与成员生命周期独立的本地直接往返观察
}

// FixedOptions implements [vivid.FixedOptionActor].
func (a *Actor) FixedOptions(ctx vivid.FixedOptionContext) []vivid.ActorOption {
	return []vivid.ActorOption{
		vivid.WithActorName(gossipActorName),
	}
}

// OnPrelaunch 在 Actor 启动前执行：创建本节点 Information 并加入本地视图的成员列表。
func (a *Actor) OnPrelaunch(ctx vivid.PrelaunchContext) error {
	if err := a.initializeSeeds(ctx); err != nil {
		return err
	}
	a.joinRequest, a.joinError = nil, nil
	a.joinIndex, a.stopping = 0, false
	a.joinAttempt, a.joinDeadline = 0, time.Time{}

	// 创建集群视图
	a.view = NewClusterView(ctx.Logger())
	a.viewRevision = 0
	a.snapshot = nil
	a.membersChanged = false
	a.reachability = newLocalReachability()

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
	case launchJoin:
		if m.instanceID == a.info.ID() && !a.stopping && a.info.Status == endpoint.StatusNone {
			changeStatus(ctx, a, EventJoining)
		}
	case joinReply:
		a.onJoinReply(ctx, m)
	case SnapshotRequest:
		a.refreshReachability(ctx, time.Now())
		ctx.Reply(a.currentSnapshot())
	case *gossipmessages.Ping:
		a.onPing(ctx, m)
	case *gossipmessages.Pong:
		a.onPong(ctx, m)
	case *gossipmessages.SpreadGossip:
		a.onSpreadGossip(ctx, m)
	case *gossipmessages.Converged:
		a.onConverged(ctx)
	case *vivid.OnKill:
		a.onKill(ctx, m)
	}
}

func (a *Actor) onStatusChanged(ctx vivid.ActorContext, status endpoint.Status) {
	ctx.Logger().Debug("status changed", log.String("status", status.String()))

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

func (a *Actor) onUp(ctx vivid.ActorContext) {
	// 注册周期任务，按 [GossipInterval] 向视图中节点发起 gossip。
	spreadGossipMessage := gossipmessages.NewSpreadGossip(gossipmessages.GossipTypeCycleSYNC)
	if err := ctx.Scheduler().Loop(ctx.Ref(), a.opts.GossipInterval, spreadGossipMessage, vivid.WithSchedulerReference(singleSchedulerReference)); err != nil {
		a.joinError = vivid.ErrorGossipScheduleFailed.With(err)
		a.readiness.complete(vivid.ClusterJoinResult{}, a.joinError)
		// 报告故障以可以支持监管者重启
		ctx.Failed(vivid.ErrorGossipScheduleFailed.With(err).WithMessage("failed to schedule spread gossip"))
	}
}

func (a *Actor) onConverged(ctx vivid.ActorContext) {
	// Removed 记录及其版本需一起保留，局部视图稳定不能证明旧消息已消失。
	// 计算新的协调者
	newCoordinator := a.view.Members().GetCoordinatorNodeID()

	// 记录协调者变化
	if newCoordinator != a.coordinatorNodeID {
		ctx.Logger().Info("coordinator changed", log.String("old", a.coordinatorNodeID), log.String("new", newCoordinator))
		a.coordinatorNodeID = newCoordinator
	}

	ctx.Logger().Debug("cluster converged",
		log.String("coordinator", a.coordinatorNodeID),
		log.Duration("convergence_duration", time.Since(a.convergenceStartedAt)),
		log.String("status", a.info.Status.String()),
	)

	// 自身状态迁移
	switch a.info.Status {
	case endpoint.StatusLeaving:
		changeStatus(ctx, a, EventExiting)
	case endpoint.StatusExiting:
		changeStatus(ctx, a, EventRemoved)
	case endpoint.StatusRemoved:
		a.completePhaseKill()
	default:
	}
}

// onPing 接收实例成员记录及版本，并回复当前视图。
func (a *Actor) onPing(ctx vivid.ActorContext, ping *gossipmessages.Ping) {
	if ping == nil || ping.Info == nil || ping.Info.ActorRef == nil || ping.MemberList == nil || ping.Version == nil {
		return
	}
	// 加入完成前不接收 Ping，避免向其他节点提供未就绪视图。
	if a.info.Status == endpoint.StatusNone || a.info.Status == endpoint.StatusJoining ||
		ping.GossipType == gossipmessages.GossipTypeJoin && a.info.Status != endpoint.StatusUp {
		return
	}
	a.refreshReachability(ctx, time.Now())

	if a.mergeMember(ping.Info, ping.Version) {
		ctx.Tell(ctx.Ref(), gossipmessages.NewSpreadGossip(gossipmessages.GossipTypeDiscoverNewNode))
	}
	for _, member := range ping.MemberList.List() {
		if member.ID() != ping.Info.ID() {
			a.mergeMember(member, ping.Version)
		}
	}
	if a.membersChanged {
		a.publishMembersChanged(ctx)
	}
	maybeEmitConverged(ctx, a)

	// 避免远程发送序列化过程中被修改。
	pong := a.preparePong(ping.GossipType)
	if ping.ProbeSequence != 0 {
		pong.ProbeSequence, pong.ProbeOriginID = ping.ProbeSequence, ping.Info.ID()
	}
	ctx.Reply(pong)
}

// onPong 接收加入请求或周期同步得到的实例视图。
func (a *Actor) onPong(ctx vivid.ActorContext, pong *gossipmessages.Pong) {
	if pong == nil || pong.Info == nil || pong.Info.ActorRef == nil || pong.MemberList == nil || pong.Version == nil {
		return
	}
	if a.info.Status == endpoint.StatusNone || a.info.Status == endpoint.StatusJoining {
		return
	}
	now := time.Now()
	a.refreshReachability(ctx, now)
	a.mergePong(ctx, pong)
	a.observePong(ctx, pong, now)
}

func (a *Actor) mergePong(ctx vivid.ActorContext, pong *gossipmessages.Pong) {
	a.mergeMember(pong.Info, pong.Version)
	for _, member := range pong.MemberList.List() {
		if member.ID() != pong.Info.ID() {
			a.mergeMember(member, pong.Version)
		}
	}
	if a.membersChanged {
		a.publishMembersChanged(ctx)
	}
	maybeEmitConverged(ctx, a)
}

// mergeMember 同时接收成员记录及其版本，返回是否发现了新实例。
func (a *Actor) mergeMember(info *endpoint.Information, version *versionvector.VersionVector) bool {
	id := info.ID()
	// 本实例是自身状态和版本的唯一写入方，远端转发不得修改本地历史。
	if id == "" || id == a.info.ID() {
		return false
	}
	remoteVersion := version.Get(id)
	// 同一实例的同一版本视为重复更新，不再按状态优先级覆盖。
	if remoteVersion <= a.view.Version().Get(id) {
		return false
	}
	isNew := a.view.Members().Upsert(info)
	a.view.Version().Update(id, remoteVersion)
	if a.reachability != nil && !observableMember(info.Status) {
		a.reachability.forget(id)
	}
	a.membersChanged = true
	return isNew
}

func (a *Actor) onKill(ctx vivid.ActorContext, _ *vivid.OnKill) {
	if a.stopping {
		return
	}
	a.stopping = true
	if a.reachability != nil {
		a.reachability.reset()
		a.publishReachabilityChanged(ctx)
	}
	a.readiness.Stop()
	if a.joinRequest != nil {
		a.joinRequest.Close(vivid.ErrorActorSystemStopped)
		a.joinRequest = nil
	}
	if ctx.Scheduler().Exists(launchSchedulerReference) {
		_ = ctx.Scheduler().Cancel(launchSchedulerReference)
	}
	unjoined := a.info.Status == endpoint.StatusNone || a.info.Status == endpoint.StatusJoining
	changeStatus(ctx, a, EventLeaving)
	if unjoined {
		// 尚未进入 Up，也没有周期 Gossip，无需等待不存在的稳定轮次。
		changeStatus(ctx, a, EventExiting)
		changeStatus(ctx, a, EventRemoved)
		a.completePhaseKill()
	}
}

func (a *Actor) completePhaseKill() {
	select {
	case <-a.phaseKillCompleted:
	default:
		close(a.phaseKillCompleted)
	}
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
	ctx.Logger().Debug("removed from cluster")
}

// onSpreadGossip 按每轮目标数限制向可联络成员及种子发送 Ping。
func (a *Actor) onSpreadGossip(ctx vivid.ActorContext, spreadGossip *gossipmessages.SpreadGossip) {
	now := time.Now()
	a.refreshReachability(ctx, now)
	observe := a.reachability != nil && !a.stopping && a.info.Status == endpoint.StatusUp
	limit := a.opts.GossipPeersCount
	if observe {
		// 观察需要看到所有候选地址，不能在分配额度前先随机截断。
		limit = len(a.view.Members().List()) + len(a.seeds)
	}
	peers := a.view.Members().Unseens(a.info, a.seeds, limit)
	if observe {
		peers = a.reachability.selectPeers(peers, a.opts.GossipPeersCount)
	}
	if len(peers) == 0 {
		maybeEmitConverged(ctx, a)
		return
	}

	// 避免远程发送序列化过程中被修改
	ping := a.preparePing(spreadGossip.GossipType)

	for _, peer := range peers {
		if observe {
			sequence := a.reachability.begin(peer, a.view.Members().List(), now, a.opts.ProbeTimeout, a.opts.GossipPeersCount)
			if sequence != 0 {
				// 多个目标共享只读视图，但各自保留独立的请求序号。
				request := *ping
				request.ProbeSequence = sequence
				ctx.Tell(peer, &request)
				continue
			}
		}
		ctx.Tell(peer, ping)
	}
}

func (a *Actor) preparePing(gossipType string) *gossipmessages.Ping {
	infoCopied := a.info.Clone()
	memberListCopied := a.view.Members().Clone()
	versionVectorCopied := a.view.Version().Clone()
	return gossipmessages.NewPing(gossipType, infoCopied, memberListCopied, versionVectorCopied)
}

func (a *Actor) preparePong(gossipType string) *gossipmessages.Pong {
	infoCopied := a.info.Clone()
	memberListCopied := a.view.Members().Clone()
	versionVectorCopied := a.view.Version().Clone()
	return gossipmessages.NewPong(gossipType, infoCopied, memberListCopied, versionVectorCopied)
}
