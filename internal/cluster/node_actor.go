package cluster

import (
	"math/rand"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/utils"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
)

var _ vivid.Actor = (*NodeActor)(nil)

func NewNodeActor(address string, actorRefParser ActorRefParser, options vivid.ClusterOptions) *NodeActor {
	state := newNodeState(options.NodeID, options.ClusterName, address)
	if state.Labels == nil {
		state.Labels = make(map[string]string)
	}
	if options.Datacenter != "" {
		state.Labels[LabelDatacenter] = options.Datacenter
	}
	if options.Rack != "" {
		state.Labels[LabelRack] = options.Rack
	}
	if options.Region != "" {
		state.Labels[LabelRegion] = options.Region
	}
	if options.Zone != "" {
		state.Labels[LabelZone] = options.Zone
	}
	cv := newClusterView()
	cv.MaxVersionVectorEntries = options.MaxVersionVectorEntries
	seedsProvider := NewSeedsProvider(options)
	rate, burst, maxEnt := ApplyJoinRateLimiterOptions(options)
	joinLimiter := NewJoinRateLimiter(rate, burst, maxEnt)
	gossipRate, gossipBurst := ApplyGossipRateLimiterOptions(options)
	gossipLimiter := NewGossipRateLimiter(gossipRate, gossipBurst)
	return &NodeActor{
		options:           options,
		actorRefParser:    actorRefParser,
		nodeState:         state,
		clusterView:       cv,
		seedsProvider:     seedsProvider,
		quorumCalc:        NewQuorumCalculator(options),
		joinRateLimiter:   joinLimiter,
		gossipRateLimiter: gossipLimiter,
		gossipSelector:    NewGossipTargetSelector(options, seedsProvider.GetAllSeedsWithDC),
		failureDetector:   NewFailureDetector(options),
		events:            NewClusterEventPublisher(),
		leaveCoord:        NewLeaveCoordinator(),
		metricsUpdater:    NewClusterMetricsUpdater(),
		joinBackoff:       utils.NewExponentialBackoffWithDefault(InitialJoinRetryDelay, MaxJoinRetryDelay),
	}
}

// NodeActor 集群节点 Actor，通过组合各功能组件管理加入、Gossip、故障检测、退出与事件发布。
type NodeActor struct {
	options           vivid.ClusterOptions
	actorRefParser    ActorRefParser
	nodeState         *NodeState
	clusterView       *ClusterView
	seedsProvider     *SeedsProvider
	quorumCalc        *QuorumCalculator
	joinRateLimiter   *JoinRateLimiter
	gossipRateLimiter *GossipRateLimiter
	gossipSelector    *GossipTargetSelector
	failureDetector   *FailureDetector
	events            *ClusterEventPublisher
	leaveCoord        *LeaveCoordinator
	metricsUpdater    *ClusterMetricsUpdater
	joinBackoff       *utils.ExponentialBackoff
}

func (a *NodeActor) OnReceive(ctx vivid.ActorContext) {
	switch m := ctx.Message().(type) {
	case *vivid.OnLaunch:
		a.onLaunch(ctx)
	case *JoinRetryTick:
		a.onJoinRetryTick(ctx)
	case *LeaveRequest:
		if a.nodeState.Status == MemberStatusJoining {
			a.onLeaveWhileJoining(ctx)
		} else {
			a.handleLeaveRequest(ctx)
		}
	case *JoinRequest:
		a.handleJoinRequest(ctx, m)
	case *GossipMessage:
		a.handleGossip(ctx, m)
	case *GossipTick:
		a.runGossipRound(ctx)
	case *GossipCrossDCTick:
		a.runGossipRoundCrossDC(ctx)
	case *FailureDetectionTick:
		a.runFailureDetection(ctx)
	case *GetViewRequest:
		a.handleGetView(ctx)
	case *ForceMemberDown:
		a.handleForceMemberDown(ctx, m)
	case *TriggerViewBroadcast:
		a.handleTriggerBroadcast(ctx, m)
	case *ExitingReady:
		if a.nodeState != nil {
			a.nodeState.Status = MemberStatusExiting
			ctx.Logger().Debug("cluster node exiting", log.String("nodeId", a.nodeState.ID))
		}
		if ref := a.leaveCoord.GetAndClearReplyTo(); ref != nil {
			ctx.Tell(ref, &LeaveAck{})
		}
		a.publishLeaveCompleted(ctx)
	}
}

func (a *NodeActor) onLaunch(ctx vivid.ActorContext) {
	seeds := a.seedsProvider.GetSeedsForJoin(a.nodeState.Datacenter())
	if len(seeds) == 0 || a.isSelfInSeeds(seeds) {
		a.bootstrapAsSeed(ctx)
		a.startGossipLoop(ctx)
		a.startFailureDetectionLoop(ctx)
		return
	}
	if err := a.tryJoinSeeds(ctx, seeds); err == nil {
		_ = ctx.Scheduler().Cancel(SchedRefJoinRetry)
		a.joinBackoff.Reset()
		ctx.Logger().Debug("joined cluster")
		a.startGossipLoop(ctx)
		a.startFailureDetectionLoop(ctx)
		return
	}
	ctx.Logger().Warn("join failed, will retry with backoff")
	a.joinBackoff.Reset()
	a.scheduleJoinRetry(ctx)
}

func (a *NodeActor) onJoinRetryTick(ctx vivid.ActorContext) {
	seeds := a.seedsProvider.GetSeedsForJoin(a.nodeState.Datacenter())
	if err := a.tryJoinSeeds(ctx, seeds); err == nil {
		_ = ctx.Scheduler().Cancel(SchedRefJoinRetry)
		a.joinBackoff.Reset()
		ctx.Logger().Debug("joined cluster after retry")
		a.startGossipLoop(ctx)
		a.startFailureDetectionLoop(ctx)
		return
	}
	nextDelay := a.scheduleJoinRetry(ctx)
	ctx.Logger().Debug("join retry failed, next in "+nextDelay.String(), log.Any("nextDelay", nextDelay))
}

func (a *NodeActor) onLeaveWhileJoining(ctx vivid.ActorContext) {
	a.cancelAllSchedulers(ctx)
	a.nodeState.Status = MemberStatusExiting
	ctx.TellSelf(&ExitingReady{})
}

// scheduleJoinRetry 使用指数退避调度下一次 Join 重试，返回本次使用的延迟（用于日志）。
func (a *NodeActor) scheduleJoinRetry(ctx vivid.ActorContext) time.Duration {
	delay := a.joinBackoff.Next()
	_ = ctx.Scheduler().Once(ctx.Ref(), delay, &JoinRetryTick{NextDelay: delay},
		vivid.WithSchedulerReference(SchedRefJoinRetry))
	return delay
}

// acceptProtocolVersion 判断收到的视图协议版本是否在配置的接受范围内。
func (a *NodeActor) acceptProtocolVersion(version uint16) bool {
	if a.options.MinProtocolVersion > 0 && version < a.options.MinProtocolVersion {
		return false
	}
	if a.options.MaxProtocolVersion > 0 && version > a.options.MaxProtocolVersion {
		return false
	}
	return true
}

func (a *NodeActor) getMergeOptions() MergeOptions {
	return MergeOptions{
		MaxClockSkew:              a.options.MaxClockSkew,
		VersionConcurrentStrategy: int(a.options.VersionConcurrentStrategy),
	}
}

func (a *NodeActor) isSelfInSeeds(seeds []string) bool {
	self, ok := utils.NormalizeAddress(a.nodeState.Address)
	if !ok {
		return false
	}
	for _, s := range seeds {
		if s == self {
			return true
		}
	}
	return false
}

func (a *NodeActor) bootstrapAsSeed(ctx vivid.ActorContext) {
	a.nodeState.Status = MemberStatusUp
	a.clusterView.AddMember(a.nodeState)
	a.clusterView.IncrementVersion(a.nodeState.ID)
	a.events.PublishLeaderIfChanged(ctx, a.clusterView, a.nodeState.Address, a.quorumCalc.SatisfiesQuorum(a.clusterView))
	a.metricsUpdater.Update(ctx, a.clusterView)
	a.broadcastViewOnce(ctx)
	ctx.Logger().Debug("started as seed node", log.String("address", a.nodeState.Address), log.String("nodeId", a.nodeState.ID))
}

func (a *NodeActor) tryJoinSeeds(ctx vivid.ActorContext, seeds []string) error {
	if len(seeds) == 0 {
		return vivid.ErrorIllegalArgument
	}
	if a.nodeState == nil {
		return vivid.ErrorIllegalArgument
	}
	shuffled := make([]string, len(seeds))
	copy(shuffled, seeds)
	rand.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	req := &JoinRequest{NodeState: a.nodeState.Clone()}
	if a.options.JoinSecret != "" {
		req.AuthToken = ComputeJoinToken(a.options.JoinSecret, a.nodeState)
	}
	var lastErr error
	for _, seed := range shuffled {
		ref, err := a.actorRefParser(seed, "/@cluster")
		if err != nil {
			ctx.Logger().Warn("create seed ref failed", log.String("seed", seed), log.Any("error", err))
			lastErr = err
			continue
		}
		reply, err := ctx.Ask(ref, req, JoinAskTimeout(a.options)).Result()
		if err != nil {
			ctx.Logger().Warn("join seed failed", log.String("seed", seed), log.Any("error", err))
			lastErr = err
			continue
		}
		resp, ok := reply.(*JoinResponse)
		if !ok || resp.View == nil {
			continue
		}
		if !a.acceptProtocolVersion(resp.View.ProtocolVersion) {
			lastErr = vivid.ErrorClusterProtocolVersionMismatch
			continue
		}
		a.nodeState.Status = MemberStatusUp
		a.clusterView.AddMember(a.nodeState)
		a.clusterView.IncrementVersion(a.nodeState.ID)
		a.clusterView.MergeFromWithOptions(resp.View, a.getMergeOptions())
		// 重启分代：若视图中已有本节点（例如上次离开后重启再入群），采用更高分代以便他节点采纳新实例
		if prev := a.clusterView.Members[a.nodeState.ID]; prev != nil && prev.Generation >= a.nodeState.Generation {
			a.nodeState.Generation = prev.Generation + 1
			a.nodeState.Timestamp = time.Now().UnixNano()
			if prev.LogicalClock != 0 {
				a.nodeState.LogicalClock = prev.LogicalClock + 1
			} else {
				a.nodeState.LogicalClock = 1
			}
			a.clusterView.AddMember(a.nodeState)
		}
		a.events.PublishLeaderIfChanged(ctx, a.clusterView, a.nodeState.Address, a.quorumCalc.SatisfiesQuorum(a.clusterView))
		a.broadcastViewOnce(ctx)
		return nil
	}
	return lastErr
}

func (a *NodeActor) cancelAllSchedulers(ctx vivid.ActorContext) {
	_ = ctx.Scheduler().Cancel(SchedRefGossip)
	_ = ctx.Scheduler().Cancel(SchedRefGossipCrossDC)
	_ = ctx.Scheduler().Cancel(SchedRefFailureDetection)
	_ = ctx.Scheduler().Cancel(SchedRefJoinRetry)
	_ = ctx.Scheduler().Cancel(SchedRefLeaveDelay)
}

func (a *NodeActor) handleGetView(ctx vivid.ActorContext) {
	if a.clusterView == nil {
		ctx.Reply(vivid.ErrorIllegalArgument)
		return
	}
	snap := a.clusterView.Snapshot()
	if snap == nil {
		ctx.Reply(vivid.ErrorIllegalArgument)
		return
	}
	ctx.Reply(&GetViewResponse{View: snap, InQuorum: a.quorumCalc.SatisfiesQuorum(a.clusterView)})
}

func (a *NodeActor) handleForceMemberDown(ctx vivid.ActorContext, m *ForceMemberDown) {
	if m == nil {
		return
	}
	if a.options.AdminSecret != "" && !VerifyAdminToken(a.options.AdminSecret, m.AdminToken) {
		ctx.Reply(vivid.ErrorClusterAdminAuthFailed)
		return
	}
	if a.clusterView == nil || m.NodeID == "" {
		ctx.Reply(vivid.ErrorIllegalArgument)
		return
	}
	member := a.clusterView.Members[m.NodeID]
	if member == nil {
		ctx.Reply(nil)
		return
	}
	removedAddr := member.Address
	a.clusterView.RemoveMember(m.NodeID)
	a.clusterView.IncrementVersion(a.nodeState.ID)
	a.events.PublishMembersChanged(ctx, a.memberAddresses(), 0, []string{removedAddr})
	a.events.PublishViewChanged(ctx, a.clusterView, 0, []string{removedAddr})
	a.events.PublishLeaderIfChanged(ctx, a.clusterView, a.nodeState.Address, a.quorumCalc.SatisfiesQuorum(a.clusterView))
	a.metricsUpdater.Update(ctx, a.clusterView)
	ctx.Logger().Debug("member force down", log.String("nodeId", m.NodeID), log.String("address", removedAddr))
	a.broadcastViewOnce(ctx)
	ctx.Reply(nil)
}

func (a *NodeActor) handleTriggerBroadcast(ctx vivid.ActorContext, m *TriggerViewBroadcast) {
	if a.options.AdminSecret != "" {
		if m == nil || !VerifyAdminToken(a.options.AdminSecret, m.AdminToken) {
			ctx.Reply(vivid.ErrorClusterAdminAuthFailed)
			return
		}
	}
	a.broadcastViewOnce(ctx)
	ctx.Reply(nil)
}

func (a *NodeActor) handleJoinRequest(ctx vivid.ActorContext, m *JoinRequest) {
	if m == nil || m.NodeState == nil {
		ctx.Reply(vivid.ErrorIllegalArgument)
		return
	}
	senderAddr := ""
	if ctx.Sender() != nil {
		senderAddr = ctx.Sender().GetAddress()
	}
	if !a.joinRateLimiter.Allow(senderAddr) {
		ctx.Reply(vivid.ErrorClusterJoinRateLimited)
		return
	}
	if len(a.options.JoinAllowAddresses) > 0 && !AllowJoinByAddress(senderAddr, a.options.JoinAllowAddresses) {
		ctx.Reply(vivid.ErrorClusterJoinNotAllowed)
		return
	}
	if len(a.options.JoinAllowDCs) > 0 && !AllowJoinByDC(m.NodeState.Datacenter(), a.options.JoinAllowDCs) {
		ctx.Reply(vivid.ErrorClusterJoinNotAllowed)
		return
	}
	if a.options.ClusterName != "" && m.NodeState.ClusterName != a.options.ClusterName {
		ctx.Reply(vivid.ErrorClusterNameMismatch)
		return
	}
	if m.NodeState.Status != MemberStatusJoining {
		ctx.Reply(vivid.ErrorClusterNodeStatusMismatch)
		return
	}
	if a.options.JoinSecret != "" && !VerifyJoinToken(a.options.JoinSecret, m.AuthToken, m.NodeState) {
		ctx.Reply(vivid.ErrorClusterJoinAuthFailed)
		return
	}
	if !a.quorumCalc.SatisfiesQuorum(a.clusterView) {
		ctx.Reply(vivid.ErrorClusterNotInQuorum)
		return
	}
	accepted := m.NodeState.Clone()
	accepted.Status = MemberStatusUp
	a.clusterView.AddMember(accepted)
	a.clusterView.IncrementVersion(a.nodeState.ID)
	a.events.PublishMembersChanged(ctx, a.memberAddresses(), 1, nil)
	a.events.PublishLeaderIfChanged(ctx, a.clusterView, a.nodeState.Address, a.quorumCalc.SatisfiesQuorum(a.clusterView))
	a.metricsUpdater.Update(ctx, a.clusterView)
	snap := a.clusterView.Snapshot()
	if snap == nil {
		ctx.Reply(vivid.ErrorIllegalArgument)
		return
	}
	ctx.Reply(&JoinResponse{View: snap})
	a.broadcastViewOnce(ctx) // 状态变更立即同步
}

func (a *NodeActor) handleGossip(ctx vivid.ActorContext, m *GossipMessage) {
	if m == nil || m.View == nil {
		return
	}
	if a.clusterView == nil {
		return
	}
	if !a.acceptProtocolVersion(m.View.ProtocolVersion) {
		ctx.Logger().Debug("gossip view rejected: protocol version out of range", log.Any("version", m.View.ProtocolVersion))
		return
	}
	a.metricsUpdater.UpdateViewDivergence(ctx, a.clusterView, m.View)
	sender := ctx.Sender()
	if sender != nil {
		if addr := sender.GetAddress(); addr != "" {
			if member := a.clusterView.MemberByAddress(addr); member != nil {
				member.LastSeen = time.Now().UnixNano()
				if member.Status == MemberStatusSuspect {
					member.Status = MemberStatusUp
				}
			}
		}
	}
	a.clusterView.MergeFromWithOptions(m.View, a.getMergeOptions())
	a.events.PublishLeaderIfChanged(ctx, a.clusterView, a.nodeState.Address, a.quorumCalc.SatisfiesQuorum(a.clusterView))
	a.broadcastViewOnce(ctx)
}

// runGossipRound 执行一轮 Gossip，由调度器周期触发。
func (a *NodeActor) runGossipRound(ctx vivid.ActorContext) {
	targets := a.gossipSelector.SelectTargets(a.clusterView, a.nodeState)
	snap := a.clusterView.Snapshot()
	if snap == nil {
		return
	}
	msg := &GossipMessage{View: snap}
	for _, addr := range targets {
		if !a.gossipRateLimiter.Allow() {
			break
		}
		ref, err := a.actorRefParser(addr, "/@cluster")
		if err != nil {
			continue
		}
		ctx.Tell(ref, msg)
	}
}

func (a *NodeActor) runGossipRoundCrossDC(ctx vivid.ActorContext) {
	targets := a.gossipSelector.SelectTargetsCrossDC(a.clusterView, a.nodeState)
	if len(targets) == 0 {
		return
	}
	snap := a.clusterView.Snapshot()
	if snap == nil {
		return
	}
	msg := &GossipMessage{View: snap}
	for _, addr := range targets {
		if !a.gossipRateLimiter.Allow() {
			break
		}
		ref, err := a.actorRefParser(addr, "/@cluster")
		if err != nil {
			continue
		}
		ctx.Tell(ref, msg)
	}
}

func (a *NodeActor) startGossipLoop(ctx vivid.ActorContext) {
	interval := a.options.DiscoveryInterval
	if interval <= 0 {
		interval = DefaultGossipInterval
	}
	_ = ctx.Scheduler().Loop(ctx.Ref(), interval, &GossipTick{},
		vivid.WithSchedulerReference(SchedRefGossip))
	if a.options.CrossDCDiscoveryInterval > 0 {
		_ = ctx.Scheduler().Loop(ctx.Ref(), a.options.CrossDCDiscoveryInterval, &GossipCrossDCTick{},
			vivid.WithSchedulerReference(SchedRefGossipCrossDC))
	}
}

func (a *NodeActor) runFailureDetection(ctx vivid.ActorContext) {
	if a.clusterView == nil {
		return
	}
	now := time.Now()
	toSuspect, toRemove := a.failureDetector.RunDetection(a.clusterView, a.nodeState.Address, a.nodeState.Datacenter(), now)
	for _, id := range toSuspect {
		if m := a.clusterView.Members[id]; m != nil {
			ctx.Logger().Debug("member suspect", log.String("nodeId", id), log.String("address", m.Address))
			m.Status = MemberStatusSuspect
			a.clusterView.IncrementVersion(a.nodeState.ID)
		}
	}
	if len(toSuspect) > 0 {
		a.events.PublishViewChanged(ctx, a.clusterView, 0, nil)
		a.broadcastViewOnce(ctx)
	}
	removedAddrs := make([]string, 0, len(toRemove))
	for _, id := range toRemove {
		if m := a.clusterView.Members[id]; m != nil {
			removedAddrs = append(removedAddrs, m.Address)
			ctx.Logger().Debug("member unreachable, removing", log.String("nodeId", id), log.String("address", m.Address))
		}
		a.clusterView.RemoveMember(id)
		a.clusterView.IncrementVersion(a.nodeState.ID)
	}
	if len(removedAddrs) > 0 {
		a.events.PublishMembersChanged(ctx, a.memberAddresses(), 0, removedAddrs)
		a.events.PublishViewChanged(ctx, a.clusterView, 0, removedAddrs)
		a.broadcastViewOnce(ctx)
	}
	inQuorum := a.quorumCalc.SatisfiesQuorum(a.clusterView)
	a.events.PublishLeaderIfChanged(ctx, a.clusterView, a.nodeState.Address, inQuorum)
	a.metricsUpdater.Update(ctx, a.clusterView)
	a.events.PublishDCHealthChangedIfNeeded(ctx, a.clusterView)
	seeds := a.seedsProvider.GetSeedsForJoin(a.nodeState.Datacenter())
	if !inQuorum && len(seeds) > 0 {
		a.tryQuorumRecovery(ctx)
	}
}

func (a *NodeActor) tryQuorumRecovery(ctx vivid.ActorContext) {
	if a.clusterView == nil {
		return
	}
	seeds := a.seedsProvider.GetSeedsForJoin(a.nodeState.Datacenter())
	req := &GetViewRequest{}
	n := MaxGetViewTargets
	if n > len(seeds) {
		n = len(seeds)
	}
	for i := 0; i < n; i++ {
		ref, err := a.actorRefParser(seeds[i], "/@cluster")
		if err != nil {
			continue
		}
		reply, err := ctx.Ask(ref, req, GetViewAskTimeout(a.options)).Result()
		if err != nil {
			continue
		}
		if resp, ok := reply.(*GetViewResponse); ok && resp.View != nil {
			if !a.acceptProtocolVersion(resp.View.ProtocolVersion) {
				continue
			}
			a.clusterView.MergeFromWithOptions(resp.View, a.getMergeOptions())
			a.metricsUpdater.Update(ctx, a.clusterView)
			a.broadcastViewOnce(ctx)
			ctx.Logger().Debug("quorum recovery: merged view from seed", log.String("seed", seeds[i]))
			return
		}
	}
}

func (a *NodeActor) startFailureDetectionLoop(ctx vivid.ActorContext) {
	timeout := a.options.FailureDetectionTimeout
	if timeout <= 0 {
		return
	}
	interval := timeout / 2
	if interval < time.Second {
		interval = time.Second
	}
	_ = ctx.Scheduler().Loop(ctx.Ref(), interval, &FailureDetectionTick{},
		vivid.WithSchedulerReference(SchedRefFailureDetection))
}

func (a *NodeActor) handleLeaveRequest(ctx vivid.ActorContext) {
	if a.nodeState == nil {
		ctx.Reply(nil)
		return
	}
	sender := ctx.Sender()
	if a.nodeState.Status == MemberStatusLeaving || a.nodeState.Status == MemberStatusExiting {
		if sender != nil {
			ctx.Tell(sender, &LeaveAck{})
		}
		return
	}
	a.leaveCoord.SetReplyTo(sender)
	a.cancelAllSchedulers(ctx)
	a.nodeState.Status = MemberStatusLeaving
	a.broadcastViewOnce(ctx)
	a.nodeState.Status = MemberStatusExiting
	ctx.Logger().Debug("cluster node exiting", log.String("nodeId", a.nodeState.ID))
	if ref := a.leaveCoord.GetAndClearReplyTo(); ref != nil {
		ctx.Reply(&LeaveAck{})
	}
	a.publishLeaveCompleted(ctx)
}

func (a *NodeActor) publishLeaveCompleted(ctx vivid.ActorContext) {
	if ctx == nil {
		return
	}
	es := ctx.EventStream()
	if es == nil {
		return
	}
	es.Publish(ctx, ves.ClusterLeaveCompletedEvent{NodeRef: ctx.Ref()})
}

func (a *NodeActor) broadcastViewOnce(ctx vivid.ActorContext) {
	snap := a.clusterView.Snapshot()
	if snap == nil {
		return
	}
	msg := &GossipMessage{View: snap}
	for _, addr := range a.gossipSelector.SelectTargets(a.clusterView, a.nodeState) {
		if !a.gossipRateLimiter.Allow() {
			break
		}
		ref, err := a.actorRefParser(addr, "/@cluster")
		if err != nil {
			continue
		}
		ctx.Tell(ref, msg)
	}
}

func (a *NodeActor) memberAddresses() []string {
	if a.clusterView == nil || a.clusterView.Members == nil {
		return nil
	}
	out := make([]string, 0, len(a.clusterView.Members))
	for _, m := range a.clusterView.Members {
		if m != nil && m.Address != "" {
			out = append(out, m.Address)
		}
	}
	return out
}
