package cluster

import (
	"math/rand"
	"sort"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/utils"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
)

const (
	discoverySchedulerRef  = "cluster-discovery"
	discoveryJitterPercent = 20 // 每轮间隔的随机抖动上限（%），用于错峰，减轻流量尖峰
)

var (
	_ vivid.Actor          = (*NodeActor)(nil)
	_ vivid.PrelaunchActor = (*NodeActor)(nil)
)

// ActorRefParser 用于解析 ActorRef 的函数类型。避免内部包循环依赖。
type ActorRefParser = func(address, path string) (vivid.ActorRef, error)

func NewNodeActor(actorRefParser ActorRefParser, options vivid.ClusterOptions) *NodeActor {
	return &NodeActor{options: options, actorRefParser: actorRefParser}
}

// NodeActor 维护本节点的集群成员视图、种子列表与选主结果；串行处理，无需锁。
// 持有 options 并直接使用，与 System 持有 ActorSystemOptions 一致。
type NodeActor struct {
	options              vivid.ClusterOptions    // 集群选项
	actorRefParser       ActorRefParser          // 用于解析 ActorRef 的函数，避免内部包循环依赖。
	nodeVersion          string                  // 节点版本
	selfAddr             string                  // 本节点地址
	normalizedSeeds      []string                // 由 options.Seeds 在 onLaunch 时归一化并缓存
	members              map[string]*memberState // key: normalized address
	lastLeaderAddr       string                  // 上一轮选出的 Leader 地址
	lastInQuorum         bool                    // 上一轮是否处于多数派，用于检测 InQuorum 变化并发布事件
	lastKnownClusterSize int                     // 历史上出现过的最大成员数（合并时增大，仅显式 Leave 时缩小），用于多数派判定
}

func (a *NodeActor) OnReceive(ctx vivid.ActorContext) {
	switch msg := ctx.Message().(type) {
	case *vivid.OnLaunch:
		a.onLaunch(ctx)
	case *nodeMessageAsGetNodesRequest:
		a.handleGetNodesRequest(ctx, msg)
	case *nodeMessageAsGetNodesResponse:
		a.handleGetNodesResponse(ctx, msg)
	case *nodeMessageAsLeaveCluster:
		a.handleLeaveCluster(ctx)
	case *publicMessageAsSetNodeVersion:
		a.handleSetNodeVersion(ctx, msg)
	case *publicMessageAsGetNodesQuery:
		a.handleGetNodesQuery(ctx, msg)
	case *publicMessageAsMembersUpdated:
		a.handleMembersUpdated(ctx, msg)
	case *publicMessageAsInitiateLeave:
		a.handleInitiateLeave(ctx)
	case *publicMessageAsGetClusterState:
		a.handleGetClusterState(ctx)
	case *discoverTick:
		a.handleDiscoverTick(ctx)
	}
}

func (a *NodeActor) OnPrelaunch(_ vivid.PrelaunchContext) error {
	return nil
}

func (a *NodeActor) onLaunch(ctx vivid.ActorContext) {
	addr := ctx.Ref().GetAddress()
	if normalized, ok := utils.NormalizeAddress(addr); ok {
		a.selfAddr = normalized
	} else {
		a.selfAddr = addr
	}
	a.normalizedSeeds = normalizeSeeds(a.options.Seeds)
	if a.members == nil {
		a.members = make(map[string]*memberState)
	}
	now := time.Now()
	a.members[a.selfAddr] = &memberState{Address: a.selfAddr, Version: a.nodeVersion, LastSeen: now}
	if a.lastKnownClusterSize < len(a.members) {
		a.lastKnownClusterSize = len(a.members)
	}

	ctx.Logger().Debug("cluster node launched",
		log.String("self_addr", a.selfAddr),
		log.String("path", ctx.Ref().GetPath()),
		log.String("cluster_name", a.options.ClusterName),
		log.String("node_version", a.nodeVersion))

	a.updateLeaderAndPublish(ctx)
	// 启动后立即触发一轮发现，尽快与种子/已有成员同步；后续轮次由 handleDiscoverTick 按 DiscoveryInterval 调度。
	if a.options.DiscoveryInterval > 0 {
		ctx.TellSelf(discoverTickMessage)
	}
}

func (a *NodeActor) scheduleDiscovery(ctx vivid.ActorContext) {
	interval := a.options.DiscoveryInterval
	if interval <= 0 {
		return
	}
	_ = ctx.Scheduler().Cancel(discoverySchedulerRef)
	delay := interval + discoveryJitter(interval)
	if err := ctx.Scheduler().Once(ctx.Ref(), delay, discoverTickMessage, vivid.WithSchedulerReference(discoverySchedulerRef)); err != nil {
		ctx.Logger().Warn("cluster discovery scheduler start failed", log.Any("err", err))
	}
}

// discoveryJitter 返回 [0, interval*discoveryJitterPercent/100) 的随机抖动，用于错峰。
func discoveryJitter(interval time.Duration) time.Duration {
	if interval <= 0 {
		return 0
	}
	j := int64(interval) * int64(discoveryJitterPercent) / 100
	if j <= 0 {
		return 0
	}
	return time.Duration(rand.Int63n(j))
}

func (a *NodeActor) handleGetNodesRequest(ctx vivid.ActorContext, req *nodeMessageAsGetNodesRequest) {
	sender := ctx.Sender()
	if sender == nil {
		return
	}
	// 配置了集群名且与请求不一致时不回复，避免向其他集群泄露成员列表
	clusterName := a.options.ClusterName
	if clusterName != "" && req.ClusterName != "" && req.ClusterName != clusterName {
		return
	}
	// 请求方主动联系本节点，说明其已加入集群，将其加入成员视图以保证双向发现
	senderAddr := sender.GetAddress()
	if addr, ok := utils.NormalizeAddress(senderAddr); ok && addr != a.selfAddr {
		a.mergeMembers(ctx, []vivid.ClusterMemberInfo{{Address: addr, Version: ""}}, senderAddr)
		a.removeStaleMembers(ctx)
		a.updateLeaderAndPublish(ctx)
	}
	resp := &nodeMessageAsGetNodesResponse{
		ClusterName: clusterName,
		Members:     a.membersToSlice(),
	}
	ctx.Tell(sender, resp)
}

func (a *NodeActor) handleGetNodesResponse(ctx vivid.ActorContext, msg *nodeMessageAsGetNodesResponse) {
	if a.options.ClusterName != "" && msg.ClusterName != a.options.ClusterName {
		return
	}
	fromAddr := ""
	if sender := ctx.Sender(); sender != nil {
		fromAddr = sender.GetAddress()
	}
	added := a.mergeMembers(ctx, msg.Members, fromAddr)
	a.removeStaleMembers(ctx)
	a.updateLeaderAndPublish(ctx)
	if added > 0 {
		ctx.Logger().Debug("cluster members merged from response",
			log.Int("added", added),
			log.Int("total", len(a.members)))
	}
}

func (a *NodeActor) handleSetNodeVersion(ctx vivid.ActorContext, msg *publicMessageAsSetNodeVersion) {
	a.nodeVersion = msg.version
	if s, ok := a.members[a.selfAddr]; ok {
		s.Version = a.nodeVersion
	}
	ctx.Logger().Debug("cluster node version updated", log.String("version", a.nodeVersion))
}

func (a *NodeActor) handleGetNodesQuery(ctx vivid.ActorContext, msg *publicMessageAsGetNodesQuery) {
	if a.options.ClusterName != "" && msg.ClusterName != a.options.ClusterName {
		ctx.Reply(vivid.ErrorClusterNameMismatch)
		return
	}
	ctx.Reply(&publicMessageAsGetNodesResult{Members: a.membersToSlice()})
}

func (a *NodeActor) handleGetClusterState(ctx vivid.ActorContext) {
	inQuorum := a.hasQuorum()
	ctx.Reply(&publicMessageAsGetClusterStateResult{
		LeaderAddress: a.lastLeaderAddr,
		InQuorum:      inQuorum,
	})
}

func (a *NodeActor) handleMembersUpdated(ctx vivid.ActorContext, msg *publicMessageAsMembersUpdated) {
	if len(msg.nodes) == 0 {
		return
	}
	infos := make([]vivid.ClusterMemberInfo, 0, len(msg.nodes))
	for _, addr := range msg.nodes {
		infos = append(infos, vivid.ClusterMemberInfo{Address: addr, Version: ""})
	}
	added := a.mergeMembers(ctx, infos, "")
	if added > 0 {
		ctx.Logger().Debug("cluster members updated from provider", log.Int("added", added))
	}
	a.updateLeaderAndPublish(ctx)
}

// handleLeaveCluster 发送方主动离开，将其从成员表移除并发布事件、更新选主。
func (a *NodeActor) handleLeaveCluster(ctx vivid.ActorContext) {
	sender := ctx.Sender()
	if sender == nil {
		return
	}
	addr, ok := utils.NormalizeAddress(sender.GetAddress())
	if !ok || addr == a.selfAddr {
		return
	}
	if _, ok := a.members[addr]; !ok {
		return
	}
	delete(a.members, addr)
	a.lastKnownClusterSize = len(a.members)
	if a.lastKnownClusterSize <= 0 {
		a.lastKnownClusterSize = 0
	}
	ctx.EventStream().Publish(ctx, ves.ClusterMembersChangedEvent{
		NodeRef:    ctx.Ref(),
		Members:    a.memberAddresses(),
		AddedNum:   0,
		RemovedNum: 1,
		Removed:    []string{addr},
	})
	a.updateLeaderAndPublish(ctx)
	ctx.Logger().Debug("cluster member left", log.String("addr", addr))
}

// handleInitiateLeave 本节点主动离开：向当前已知全部成员广播 LeaveCluster，然后从本地成员表移除自身。
func (a *NodeActor) handleInitiateLeave(ctx vivid.ActorContext) {
	allAddrs := a.memberAddresses()
	var targets []string
	for _, addr := range allAddrs {
		if addr != a.selfAddr && addr != "" {
			targets = append(targets, addr)
		}
	}
	req := &nodeMessageAsLeaveCluster{}
	for _, addr := range targets {
		ref, err := a.actorRefParser(addr, ctx.Ref().GetPath())
		if err != nil {
			continue
		}
		ctx.Tell(ref, req)
	}
	delete(a.members, a.selfAddr)
	a.lastKnownClusterSize = len(a.members)
	if a.lastKnownClusterSize <= 0 {
		a.lastKnownClusterSize = 0
	}
	a.updateLeaderAndPublish(ctx)
	ctx.Logger().Debug("cluster node initiated leave", log.String("self_addr", a.selfAddr))
}

// handleDiscoverTick 先做故障剔除与选主，再向「种子 + 当前成员」发送 GetNodesRequest，
// 使视图在全体间充分传播，保证最终一致：各节点使用相同剔除规则与超时，经多轮交换后收敛到同一存活集。
func (a *NodeActor) handleDiscoverTick(ctx vivid.ActorContext) {
	a.removeStaleMembers(ctx)
	a.updateLeaderAndPublish(ctx)

	targets := a.discoveryTargets()
	req := &nodeMessageAsGetNodesRequest{ClusterName: a.options.ClusterName}
	now := time.Now()
	for _, addr := range targets {
		ref, err := a.actorRefParser(addr, ctx.Ref().GetPath())
		if err != nil {
			ctx.Logger().Debug("cluster discovery parse ref failed",
				log.String("addr", addr),
				log.Any("err", err))
			continue
		}
		ctx.Tell(ref, req)
		if s, ok := a.members[addr]; ok {
			s.LastProbed = now
		}
	}
	// 无论本轮是否有目标都调度下一轮，避免无种子/无成员时发现永久停止
	interval := a.options.DiscoveryInterval
	_ = ctx.Scheduler().Cancel(discoverySchedulerRef)
	delay := interval + discoveryJitter(interval)
	if err := ctx.Scheduler().Once(ctx.Ref(), delay, discoverTickMessage, vivid.WithSchedulerReference(discoverySchedulerRef)); err != nil {
		ctx.Logger().Warn("cluster discovery scheduler start failed", log.Any("err", err))
	}
}

// discoveryTargets 返回本轮发现的目标地址集合：种子优先，非种子成员按 LastProbed 升序取最久未同步的至多 maxDiscoveryTargetsPerTick 个（≤0 不限制），
// 保证优先向未同步/最久未同步的节点同步，在超时前每个成员都能被探测到。
func (a *NodeActor) discoveryTargets() []string {
	seedSet := make(map[string]struct{})
	for _, addr := range a.normalizedSeeds {
		seedSet[addr] = struct{}{}
	}
	var nonSeedMembers []string
	for addr := range a.members {
		if addr != a.selfAddr && addr != "" {
			if _, isSeed := seedSet[addr]; !isSeed {
				nonSeedMembers = append(nonSeedMembers, addr)
			}
		}
	}
	maxDiscoveryTargetsPerTick := a.options.MaxDiscoveryTargetsPerTick
	seedList := a.normalizedSeeds
	if maxDiscoveryTargetsPerTick > 0 && len(seedList)+len(nonSeedMembers) > maxDiscoveryTargetsPerTick {
		// 种子全保留，非种子成员按 LastProbed 升序选最久未同步的 need 个，确保在超时前每个节点都能被探测到
		need := maxDiscoveryTargetsPerTick - len(seedList)
		if need <= 0 {
			return seedList
		}
		sort.Slice(nonSeedMembers, func(i, j int) bool {
			ti, tj := a.members[nonSeedMembers[i]].LastProbed, a.members[nonSeedMembers[j]].LastProbed
			return ti.Before(tj)
		})
		if need > len(nonSeedMembers) {
			need = len(nonSeedMembers)
		}
		out := make([]string, 0, len(seedList)+need)
		out = append(out, seedList...)
		out = append(out, nonSeedMembers[:need]...)
		return out
	}
	out := make([]string, 0, len(seedList)+len(nonSeedMembers))
	out = append(out, seedList...)
	seen := make(map[string]struct{})
	for _, s := range seedList {
		seen[s] = struct{}{}
	}
	for _, addr := range nonSeedMembers {
		if _, ok := seen[addr]; !ok {
			seen[addr] = struct{}{}
			out = append(out, addr)
		}
	}
	return out
}

// mergeMembers 将 MemberInfo 列表并入 members。仅对本次响应的发送方 fromAddr 刷新 LastSeen，
// 其余成员只做并集与 Version 更新，不刷新 LastSeen，这样只有「曾直接回复过」的节点会被视为存活，
// 下线节点不再回复后其 LastSeen 不再更新，超时后会被 removeStaleMembers 剔除。
func (a *NodeActor) mergeMembers(ctx vivid.ActorContext, infos []vivid.ClusterMemberInfo, fromAddr string) int {
	now := time.Now()
	before := len(a.members)
	senderNormalized := ""
	if fromAddr != "" {
		senderNormalized, _ = utils.NormalizeAddress(fromAddr)
	}
	for _, mi := range infos {
		addr, ok := utils.NormalizeAddress(mi.Address)
		if !ok {
			continue
		}
		if existing, ok := a.members[addr]; ok {
			existing.Version = mi.Version
			if addr == senderNormalized {
				existing.LastSeen = now
			}
		} else {
			a.members[addr] = &memberState{Address: addr, Version: mi.Version, LastSeen: now}
		}
	}
	if senderNormalized != "" {
		if s, ok := a.members[senderNormalized]; ok {
			s.LastSeen = now
		}
	}
	added := len(a.members) - before
	if added > 0 {
		if len(a.members) > a.lastKnownClusterSize {
			a.lastKnownClusterSize = len(a.members)
		}
		ctx.EventStream().Publish(ctx, ves.ClusterMembersChangedEvent{
			NodeRef:    ctx.Ref(),
			Members:    a.memberAddresses(),
			AddedNum:   added,
			RemovedNum: 0,
		})
	}
	return added
}

// removeStaleMembers 剔除超过 failureDetectionTimeout 未更新的成员（不含自身）。
func (a *NodeActor) removeStaleMembers(ctx vivid.ActorContext) {
	timeout := a.options.FailureDetectionTimeout
	if timeout <= 0 {
		return
	}
	deadline := time.Now().Add(-timeout)
	var removed []string
	for addr, s := range a.members {
		if addr == a.selfAddr {
			continue
		}
		if s.LastSeen.Before(deadline) {
			removed = append(removed, addr)
		}
	}
	for _, addr := range removed {
		delete(a.members, addr)
	}
	if len(removed) > 0 {
		ctx.EventStream().Publish(ctx, ves.ClusterMembersChangedEvent{
			NodeRef:    ctx.Ref(),
			Members:    a.memberAddresses(),
			AddedNum:   0,
			RemovedNum: len(removed),
			Removed:    removed,
		})
		ctx.Logger().Debug("cluster members removed by failure detection", log.Any("removed", removed))
	}
}

// hasQuorum 判断当前视图是否达到多数派：lastKnownClusterSize<=1 视为单节点有 quorum；
// 否则要求 len(members) > lastKnownClusterSize/2（分区时少数派无 quorum）。
func (a *NodeActor) hasQuorum() bool {
	if a.lastKnownClusterSize <= 1 {
		return true
	}
	return len(a.members) > a.lastKnownClusterSize/2
}

// updateLeaderAndPublish 按当前 members 做确定性选主（最小地址），若 LeaderAddr 或 InQuorum 变化则发布事件。
func (a *NodeActor) updateLeaderAndPublish(ctx vivid.ActorContext) {
	if len(a.members) == 0 {
		return
	}
	addrs := a.memberAddresses()
	sort.Strings(addrs)
	leaderAddr := addrs[0]
	inQuorum := a.hasQuorum()
	if leaderAddr == a.lastLeaderAddr && inQuorum == a.lastInQuorum {
		return
	}
	a.lastLeaderAddr = leaderAddr
	a.lastInQuorum = inQuorum
	ctx.EventStream().Publish(ctx, ves.ClusterLeaderChangedEvent{
		NodeRef:    ctx.Ref(),
		LeaderAddr: leaderAddr,
		IAmLeader:  a.selfAddr == leaderAddr,
		InQuorum:   inQuorum,
	})
	ctx.Logger().Debug("cluster leader changed", log.String("leader", leaderAddr), log.Bool("i_am_leader", a.selfAddr == leaderAddr), log.Bool("in_quorum", inQuorum))
}

func (a *NodeActor) membersToSlice() []vivid.ClusterMemberInfo {
	if len(a.members) == 0 {
		return nil
	}
	out := make([]vivid.ClusterMemberInfo, 0, len(a.members))
	for _, s := range a.members {
		out = append(out, vivid.ClusterMemberInfo{Address: s.Address, Version: s.Version})
	}
	return out
}

func (a *NodeActor) memberAddresses() []string {
	if len(a.members) == 0 {
		return nil
	}
	out := make([]string, 0, len(a.members))
	for addr := range a.members {
		out = append(out, addr)
	}
	return out
}
