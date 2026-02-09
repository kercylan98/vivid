package vivid

import (
	"time"

	"github.com/google/uuid"
)

// QuorumStrategy 法定人数策略：全局多数、多数 DC 参与或每 DC 至少一票。
type QuorumStrategy int

const (
	// QuorumStrategyGlobalMajority 全局多数：HealthyCount >= (HealthyCount/2)+1（当前默认）。
	QuorumStrategyGlobalMajority QuorumStrategy = iota
	// QuorumStrategyMajorityDCs 多数 DC 参与：至少 ceil(DC总数/2) 个 DC 各有至少 1 个健康节点。
	QuorumStrategyMajorityDCs
	// QuorumStrategyAtLeastOnePerDC 每 DC 至少一票：每个有成员的 DC 至少 1 个健康节点。
	QuorumStrategyAtLeastOnePerDC
)

// VersionConcurrentStrategy 在合并视图且版本向量为 VersionConcurrent 时，Epoch/Timestamp 的采纳策略。
type VersionConcurrentStrategy int

const (
	// VersionConcurrentTakeMax 取较大值（默认），分区恢复后单调进展。
	VersionConcurrentTakeMax VersionConcurrentStrategy = iota
	// VersionConcurrentPreferLocal 优先本地：不采纳对方的 Epoch/Timestamp，减少错误时钟主导。
	VersionConcurrentPreferLocal
	// VersionConcurrentPreferRemote 优先远端：强制采纳对方的 Epoch/Timestamp。
	VersionConcurrentPreferRemote
)

// SeedsResolver 用于动态解析种子地址；实现方可从 DNS、K8s 等获取。nil 时使用 ClusterOptions 的静态 Seeds/SeedsByDC。
type SeedsResolver interface {
	GetSeeds() []string
	GetSeedsByDC() map[string][]string
}

type ClusterMemberInfo struct {
	Address    string // 节点 Remoting 地址 host:port
	Version    string // 节点版本号（来自 NodeState.Version）
	Datacenter string // 数据中心标识，未配置时为空
	Rack       string // 机架标识，未配置时为空
	Region     string // 区域标识，未配置时为空
	Zone       string // 可用区标识，未配置时为空
}

// ClusterContext 供业务在运行时访问集群能力：成员列表、多数派状态、优雅退出等。
// 未启用集群时 system.Cluster() 与 ctx.Cluster() 为 nil，调用前需做 nil 判断。
type ClusterContext interface {
	// GetMembers 返回当前视图中的成员列表；未启用集群时返回 ErrorClusterDisabled。
	GetMembers() ([]ClusterMemberInfo, error)
	// InQuorum 返回当前节点是否处于多数派；未启用集群时返回 ErrorClusterDisabled。
	InQuorum() (bool, error)
	// Leave 向集群发送优雅退出请求并等待广播离开视图后返回，幂等，仅执行一次。
	Leave()
}

// ClusterOptions 封装集群节点（NodeActor）的启动期配置，所有字段均在创建时确定，设计为不可变、不在运行时修改。
//
// 通过 NewClusterOptions 与一系列 ClusterOption 函数构建；未显式设置的项将使用默认值（见各 With* 函数及常量 defaultCluster*）。
// 构建后的 *ClusterOptions 会传入 SpawnNodeActor，并由 NodeActor 持有并只读使用。
type ClusterOptions struct {
	// NodeID 本节点的唯一标识符，用于在集群中唯一标识本节点。
	NodeID string
	// ClusterName 本节点所属集群的逻辑名称。仅与 ClusterName 相同的节点会交换成员视图并视为同一集群；
	// 空串表示不校验集群名，与任意节点互通。用于多集群同网时的逻辑隔离。
	ClusterName string
	// Seeds 发现阶段使用的种子节点地址列表，格式为 "host:port"（与 Remoting 广告地址一致）。
	// 节点启动后会向 Seeds 以及已发现的成员周期性发送 GetNodesRequest，以收敛成员视图。建议至少配置 2 个以上种子以保证高可用。
	// 若 SeedsByDC 非空则加入与 Gossip 时优先使用本 DC 种子，并可与 Seeds 同时存在（合并使用）。
	Seeds []string
	// SeedsByDC 按数据中心分组的种子，key 为 DC 标识。加入时优先尝试本 DC（Datacenter）的种子，再尝试其他 DC。
	// 全球部署时建议每 DC 至少配置一种子。
	SeedsByDC map[string][]string
	// DiscoveryInterval Gossip 轮次间隔，状态变更会立即同步，此间隔为周期流言传播频率。默认 1s。
	DiscoveryInterval time.Duration
	// FailureDetectionTimeout 故障检测超时时长：若某成员在此时长内未被任何「直接响应」刷新 LastSeen，将从本地视图中剔除并发布成员变更事件。
	// 设为 ≤0 表示不自动剔除，仅依赖显式 Leave。默认 40s。
	FailureDetectionTimeout time.Duration
	// MaxDiscoveryTargetsPerTick 每轮发现中最多向多少个目标发送 GetNodesRequest；种子始终全部包含，其余从成员中按 LastProbed 优先选取。
	// ≤0 表示不限制，每轮向所有种子与成员发送。默认 20，用于控制每轮请求量。
	MaxDiscoveryTargetsPerTick int
	// Datacenter 本节点所在数据中心标识，会写入 NodeState.Labels，用于多数据中心场景下的同 DC 优先 Gossip 与跨 DC 故障超时。
	Datacenter string
	// Rack 本节点所在机架标识，会写入 NodeState.Labels，便于拓扑感知。
	Rack string
	// Region 本节点所在区域标识，会写入 NodeState.Labels；selectGossipTargets 同 Region 优先再 DC 再 Rack。
	Region string
	// Zone 本节点所在可用区标识，会写入 NodeState.Labels。
	Zone string
	// SeedsResolver 可选；非空时 GetSeeds 与 GetSeedsByDC 由此提供，用于动态发现（如 DNS、K8s）；nil 时使用静态 Seeds/SeedsByDC。
	SeedsResolver SeedsResolver
	// AdminSecret 可选；非空时管理消息（强制下线、触发广播）须携带匹配的 AdminToken，否则拒绝。
	AdminSecret string
	// JoinAllowDCs 可选；非空时仅允许这些 DC 的节点加入，空表示不按 DC 限制。
	JoinAllowDCs []string
	// JoinAllowAddresses 可选；非空时仅允许这些地址（或 CIDR）的节点发起 Join，空表示不限制。格式为 "host" 或 "host:port" 或 "CIDR"。
	JoinAllowAddresses []string
	// CrossDCFailureDetectionTimeout 跨数据中心成员的故障检测超时。若 >0 则对其它 DC 的成员使用此超时；
	// 为 0 时集群内部使用 FailureDetectionTimeout 的 2 倍作为跨 DC 超时（推荐跨 DC 显式配置为 2–3 倍同 DC 超时）。
	CrossDCFailureDetectionTimeout time.Duration
	// CrossDCDiscoveryInterval 跨 DC Gossip 的轮次间隔。若 >0 则额外启动一轮仅向跨 DC 目标发送的 Gossip，降低跨 DC 带宽与误判；0 表示不单独调度跨 DC 轮次。
	CrossDCDiscoveryInterval time.Duration
	// RequiredDCsForQuorum 必须参与 quorum 的 DC 列表；非空时这些 DC 中每个至少需有 1 个健康节点才满足 quorum，用于关键 DC 必须参与的多活。
	RequiredDCsForQuorum []string
	// GossipRateLimitPerSecond 全局 Gossip 发送速率限制（条/秒）；>0 时启用，防止跨 DC 流量尖峰。0 表示不限。
	GossipRateLimitPerSecond float64
	// GossipRateLimitBurst Gossip 限流桶容量；仅当 GossipRateLimitPerSecond >0 时有效。
	GossipRateLimitBurst int
	// MaxVersionVectorEntries 版本向量最大条目数；0 表示使用默认 65535，超大规模集群可调大。
	MaxVersionVectorEntries int
	// MaxDiscoveryTargetsPerTickCrossDC 跨 DC 每轮 Gossip 最大目标数；>0 时跨 DC 轮次使用此值，否则使用 MaxDiscoveryTargetsPerTick。
	MaxDiscoveryTargetsPerTickCrossDC int
	// SuspectConfirmDuration 故障检测 Suspect 确认时长。>0 时：首次超时仅将成员置为 Suspect，超过「超时+本时长」后再从视图剔除；=0 时保持原行为（超时即剔除）。
	// 用于减少跨 DC 高延迟下的误判，同 DC 也可启用。
	SuspectConfirmDuration time.Duration
	// LeaveBroadcastDelay 优雅退出前等待「离开视图」广播发出的时间。多 DC 时建议增大（如 1–2s）。
	LeaveBroadcastDelay time.Duration
	// LeaveBroadcastRounds 优雅退出时广播离开视图的轮数；>1 时多轮广播以提高高延迟 DC 收敛概率。
	LeaveBroadcastRounds int
	// QuorumStrategy 法定人数策略：全局多数、多数 DC 或每 DC 至少一票。
	QuorumStrategy QuorumStrategy
	// JoinSecret 加入认证共享密钥。非空时 Join 请求须携带由此生成的 AuthToken，否则拒绝加入。
	JoinSecret string
	// MinProtocolVersion 接受的最小集群协议版本；收到的视图 ProtocolVersion 低于此值将被拒绝。0 表示不校验。
	MinProtocolVersion uint16
	// MaxProtocolVersion 接受的最大集群协议版本；>0 时收到的视图 ProtocolVersion 高于此值将被拒绝，用于滚动升级时拒绝未知新版本。0 表示不校验。
	MaxProtocolVersion uint16
	// JoinRateLimitPerSecond 每地址每秒允许的 Join 请求数；>0 时启用限流，防止 Join 风暴。0 表示不限流。
	JoinRateLimitPerSecond float64
	// JoinRateLimitBurst 每地址 Join 限流桶容量（突发允许数）；仅当 JoinRateLimitPerSecond >0 时有效。
	JoinRateLimitBurst int
	// MaxClockSkew 最大允许时钟偏差；收到视图的 Timestamp 与本地差超过此值时不再采纳对方的 Timestamp/Epoch 为权威，仅合并成员。0 表示不校验。
	MaxClockSkew time.Duration
	// VersionConcurrentStrategy 当版本向量为 VersionConcurrent 时 Epoch/Timestamp 的采纳策略。
	VersionConcurrentStrategy VersionConcurrentStrategy
	// JoinAskTimeout Join 请求（向种子 Ask）的超时时长。未设置或 ≤0 时使用系统 DefaultAskTimeout；建议 1s–30s。
	JoinAskTimeout time.Duration
	// GetViewAskTimeout Quorum 恢复等 GetView 请求的超时时长。未设置或 ≤0 时使用系统 DefaultAskTimeout；建议 1s–30s。
	GetViewAskTimeout time.Duration
}

// ClusterOption 是用于配置 ClusterOptions 的函数类型，采用函数式 Option 模式，便于链式/组合配置并保持向后兼容。
type ClusterOption = func(*ClusterOptions)

// NewClusterOptions 根据默认值与传入的 Option 列表构造 *ClusterOptions。
//
// 行为与 NewActorSystemOptions 一致：先将默认 Option 置于列表首部，再创建空 ClusterOptions，最后按顺序应用列表中每个 Option；
// 因此用户传入的 Option 会覆盖同名字段的默认值。调用方通常将结果传给 SpawnNodeActor 或 internal/cluster.SpawnNodeActorWithOptions。
func NewClusterOptions(opts ...ClusterOption) *ClusterOptions {
	opts = append([]ClusterOption{
		WithClusterNodeID(uuid.NewString()),
		WithClusterDiscoveryInterval(time.Second),
		WithClusterFailureDetectionTimeout(40 * time.Second),
		WithClusterMaxDiscoveryTargetsPerTick(20),
		WithClusterLeaveBroadcastDelay(200 * time.Millisecond),
	}, opts...)

	o := &ClusterOptions{}
	for _, f := range opts {
		f(o)
	}
	return o
}

// WithClusterNodeID 返回一个 ClusterOption，用于设置本节点的唯一标识符（NodeID）。
func WithClusterNodeID(id string) ClusterOption {
	return func(o *ClusterOptions) {
		o.NodeID = id
	}
}

// WithClusterName 返回一个 ClusterOption，用于设置集群逻辑名（ClusterName）。
// 同名节点会互相交换成员视图；空串表示不校验，与任意节点互通。
func WithClusterName(name string) ClusterOption {
	return func(o *ClusterOptions) {
		o.ClusterName = name
	}
}

// WithClusterSeeds 返回一个 ClusterOption，用于设置种子节点地址列表（host:port）。
// 会拷贝传入的切片，调用方后续修改不会影响已构建的 ClusterOptions。
func WithClusterSeeds(addrs []string) ClusterOption {
	return func(o *ClusterOptions) {
		o.Seeds = append([]string(nil), addrs...)
	}
}

// WithClusterSeedsByDC 返回一个 ClusterOption，用于按 DC 设置种子。加入时优先尝试本 DC 种子。
// 会拷贝 map 与各切片，调用方后续修改不会影响已构建的 ClusterOptions。
func WithClusterSeedsByDC(seedsByDC map[string][]string) ClusterOption {
	return func(o *ClusterOptions) {
		if seedsByDC == nil {
			o.SeedsByDC = nil
			return
		}
		o.SeedsByDC = make(map[string][]string, len(seedsByDC))
		for dc, addrs := range seedsByDC {
			o.SeedsByDC[dc] = append([]string(nil), addrs...)
		}
	}
}

// WithClusterDiscoveryInterval 返回一个 ClusterOption，用于设置发现轮次间隔。
// 仅当 d > 0 时生效；零或负值会被忽略，保留 NewClusterOptions 中已有的默认值。
func WithClusterDiscoveryInterval(d time.Duration) ClusterOption {
	return func(o *ClusterOptions) {
		if d > 0 {
			o.DiscoveryInterval = d
		}
	}
}

// WithClusterFailureDetectionTimeout 返回一个 ClusterOption，用于设置故障检测超时时长。
// 超过该时长未刷新的成员将被从本地视图中剔除。设为 ≤0 表示关闭自动剔除，仅依赖显式 Leave。
func WithClusterFailureDetectionTimeout(d time.Duration) ClusterOption {
	return func(o *ClusterOptions) {
		o.FailureDetectionTimeout = d
	}
}

// WithClusterMaxDiscoveryTargetsPerTick 返回一个 ClusterOption，用于设置每轮发现的目标数上限。
// ≤0 表示不限制，每轮向所有种子与成员发送请求；正值可用来限制每轮请求量。
func WithClusterMaxDiscoveryTargetsPerTick(max int) ClusterOption {
	return func(o *ClusterOptions) {
		o.MaxDiscoveryTargetsPerTick = max
	}
}

// WithClusterDatacenter 返回一个 ClusterOption，用于设置本节点所在数据中心标识（多数据中心 / 跨 DC 部署）。
func WithClusterDatacenter(dc string) ClusterOption {
	return func(o *ClusterOptions) {
		o.Datacenter = dc
	}
}

// WithClusterRack 返回一个 ClusterOption，用于设置本节点所在机架标识。
func WithClusterRack(rack string) ClusterOption {
	return func(o *ClusterOptions) {
		o.Rack = rack
	}
}

// WithClusterRegion 返回一个 ClusterOption，用于设置本节点所在区域标识（同 Region 优先 Gossip）。
func WithClusterRegion(region string) ClusterOption {
	return func(o *ClusterOptions) {
		o.Region = region
	}
}

// WithClusterZone 返回一个 ClusterOption，用于设置本节点所在可用区标识。
func WithClusterZone(zone string) ClusterOption {
	return func(o *ClusterOptions) {
		o.Zone = zone
	}
}

// WithClusterSeedsResolver 返回一个 ClusterOption，用于设置动态种子解析器；nil 时使用静态 Seeds/SeedsByDC。
func WithClusterSeedsResolver(r SeedsResolver) ClusterOption {
	return func(o *ClusterOptions) {
		o.SeedsResolver = r
	}
}

// WithClusterAdminSecret 返回一个 ClusterOption，用于设置管理操作所需的密钥；非空时强制下线、触发广播等须携带匹配 Token。
func WithClusterAdminSecret(secret string) ClusterOption {
	return func(o *ClusterOptions) {
		o.AdminSecret = secret
	}
}

// WithClusterJoinAllowDCs 返回一个 ClusterOption，用于限制仅允许指定 DC 的节点加入；空表示不限制。
func WithClusterJoinAllowDCs(dcs []string) ClusterOption {
	return func(o *ClusterOptions) {
		o.JoinAllowDCs = append([]string(nil), dcs...)
	}
}

// WithClusterJoinAllowAddresses 返回一个 ClusterOption，用于限制仅允许指定地址或 CIDR 的节点发起 Join；空表示不限制。
func WithClusterJoinAllowAddresses(addrs []string) ClusterOption {
	return func(o *ClusterOptions) {
		o.JoinAllowAddresses = append([]string(nil), addrs...)
	}
}

// WithClusterCrossDCFailureDetectionTimeout 返回一个 ClusterOption，用于设置跨数据中心成员的故障检测超时。
// 未设置或 ≤0 时，集群内部使用 FailureDetectionTimeout 的 2 倍作为跨 DC 超时。
func WithClusterCrossDCFailureDetectionTimeout(d time.Duration) ClusterOption {
	return func(o *ClusterOptions) {
		o.CrossDCFailureDetectionTimeout = d
	}
}

// WithClusterCrossDCDiscoveryInterval 返回一个 ClusterOption，用于设置跨 DC Gossip 的轮次间隔。
// >0 时额外以该间隔向跨 DC 目标发送 Gossip，降低跨 DC 带宽；0 表示不单独调度。
func WithClusterCrossDCDiscoveryInterval(d time.Duration) ClusterOption {
	return func(o *ClusterOptions) {
		o.CrossDCDiscoveryInterval = d
	}
}

// WithClusterRequiredDCsForQuorum 返回一个 ClusterOption，用于设置必须参与 quorum 的 DC 列表。
// 这些 DC 中每个至少需有 1 个健康节点才满足 quorum。
func WithClusterRequiredDCsForQuorum(dcs []string) ClusterOption {
	return func(o *ClusterOptions) {
		o.RequiredDCsForQuorum = append([]string(nil), dcs...)
	}
}

// WithClusterGossipRateLimit 返回一个 ClusterOption，用于限制全局 Gossip 发送速率（条/秒与桶容量）。
func WithClusterGossipRateLimit(perSecond float64, burst int) ClusterOption {
	return func(o *ClusterOptions) {
		o.GossipRateLimitPerSecond = perSecond
		o.GossipRateLimitBurst = burst
	}
}

// WithClusterMaxVersionVectorEntries 返回一个 ClusterOption，用于设置版本向量最大条目数；0 表示默认 65535。
func WithClusterMaxVersionVectorEntries(n int) ClusterOption {
	return func(o *ClusterOptions) {
		o.MaxVersionVectorEntries = n
	}
}

// WithClusterMaxDiscoveryTargetsPerTickCrossDC 返回一个 ClusterOption，用于设置跨 DC 每轮 Gossip 最大目标数。
func WithClusterMaxDiscoveryTargetsPerTickCrossDC(max int) ClusterOption {
	return func(o *ClusterOptions) {
		o.MaxDiscoveryTargetsPerTickCrossDC = max
	}
}

// WithClusterSuspectConfirmDuration 返回一个 ClusterOption，用于设置 Suspect 确认时长。
// >0 时先置为 Suspect，经过该时长后再剔除；=0 时超时即剔除（默认行为）。
func WithClusterSuspectConfirmDuration(d time.Duration) ClusterOption {
	return func(o *ClusterOptions) {
		o.SuspectConfirmDuration = d
	}
}

// WithClusterLeaveBroadcastDelay 返回一个 ClusterOption，用于设置优雅退出前等待广播的时间。多 DC 时建议 1–2s。
func WithClusterLeaveBroadcastDelay(d time.Duration) ClusterOption {
	return func(o *ClusterOptions) {
		o.LeaveBroadcastDelay = d
	}
}

// WithClusterLeaveBroadcastRounds 返回一个 ClusterOption，用于设置优雅退出时广播轮数；>1 时多轮广播以提高多 DC 收敛。
func WithClusterLeaveBroadcastRounds(rounds int) ClusterOption {
	return func(o *ClusterOptions) {
		o.LeaveBroadcastRounds = rounds
	}
}

// WithClusterQuorumStrategy 返回一个 ClusterOption，用于设置法定人数策略。
func WithClusterQuorumStrategy(s QuorumStrategy) ClusterOption {
	return func(o *ClusterOptions) {
		o.QuorumStrategy = s
	}
}

// WithClusterJoinSecret 返回一个 ClusterOption，用于设置加入认证共享密钥。
// 非空时 Join 请求须携带由 cluster.ComputeJoinToken(secret, nodeState) 生成的 AuthToken。
func WithClusterJoinSecret(secret string) ClusterOption {
	return func(o *ClusterOptions) {
		o.JoinSecret = secret
	}
}

// WithClusterProtocolVersionRange 返回一个 ClusterOption，用于设置接受的集群协议版本范围。
// min 为最小接受版本，收到的视图低于此值将拒绝；max 为最大接受版本，>0 时高于此值将拒绝（滚动升级时用）。0 表示不校验该边界。
func WithClusterProtocolVersionRange(min, max uint16) ClusterOption {
	return func(o *ClusterOptions) {
		o.MinProtocolVersion = min
		o.MaxProtocolVersion = max
	}
}

// WithClusterJoinRateLimit 返回一个 ClusterOption，用于按发送方地址限制 Join 请求速率。
// perSecond 为每地址每秒允许的 Join 数，burst 为桶容量；perSecond<=0 表示不限流。
func WithClusterJoinRateLimit(perSecond float64, burst int) ClusterOption {
	return func(o *ClusterOptions) {
		o.JoinRateLimitPerSecond = perSecond
		o.JoinRateLimitBurst = burst
	}
}

// WithClusterMaxClockSkew 返回一个 ClusterOption，用于设置最大允许时钟偏差。
// 收到视图的 Timestamp 与本地差超过此值时仍合并成员但不采纳对方的 Epoch/Timestamp。0 表示不校验。
func WithClusterMaxClockSkew(d time.Duration) ClusterOption {
	return func(o *ClusterOptions) {
		o.MaxClockSkew = d
	}
}

// WithClusterVersionConcurrentStrategy 返回一个 ClusterOption，用于设置 VersionConcurrent 时的 Epoch/Timestamp 采纳策略。
func WithClusterVersionConcurrentStrategy(s VersionConcurrentStrategy) ClusterOption {
	return func(o *ClusterOptions) {
		o.VersionConcurrentStrategy = s
	}
}

// WithClusterJoinAskTimeout 返回一个 ClusterOption，用于设置 Join 请求（向种子 Ask）的超时时长。建议 1s–30s；≤0 时使用系统 DefaultAskTimeout。
func WithClusterJoinAskTimeout(d time.Duration) ClusterOption {
	return func(o *ClusterOptions) {
		o.JoinAskTimeout = d
	}
}

// WithClusterGetViewAskTimeout 返回一个 ClusterOption，用于设置 GetView 请求（如 Quorum 恢复）的超时时长。建议 1s–30s；≤0 时使用系统 DefaultAskTimeout。
func WithClusterGetViewAskTimeout(d time.Duration) ClusterOption {
	return func(o *ClusterOptions) {
		o.GetViewAskTimeout = d
	}
}

// clusterSpawner 由 internal/cluster 在 init 中通过 RegisterClusterSpawner 注册，SpawnNodeActor 调用时用于实际创建 NodeActor。
var clusterSpawner func(system ActorSystem, opts *ClusterOptions) (ActorRef, error)

// RegisterClusterSpawner 供 internal/cluster 在 init 中调用，注册「根据 ActorSystem 与 ClusterOptions 创建 NodeActor」的函数。
// 不应在业务代码中调用；未注册时 SpawnNodeActor 会 panic。
func RegisterClusterSpawner(fn func(system ActorSystem, opts *ClusterOptions) (ActorRef, error)) {
	clusterSpawner = fn
}

// SpawnNodeActor 在给定 ActorSystem 下创建集群 NodeActor，固定路径为 /cluster/node。
//
// 通过 opts 传入启动期配置（集群名、种子、发现间隔、故障超时、每轮目标数等），内部会先调用 NewClusterOptions(opts...) 再交给已注册的 clusterSpawner。
// 使用前必须完成 cluster 包注册，否则会 panic；典型做法为在 main 或 init 中增加：
//
//	import _ "github.com/kercylan98/vivid/internal/cluster"
//
// 返回的 ActorRef 对应 /cluster/node，可用于 Tell/Ask 与集群交互（如发送 GetNodesQuery、MembersUpdated 等）。
func SpawnNodeActor(system ActorSystem, opts ...ClusterOption) (ActorRef, error) {
	if clusterSpawner == nil {
		panic("vivid: cluster not registered, add import _ \"github.com/kercylan98/vivid/internal/cluster\"")
	}
	return clusterSpawner(system, NewClusterOptions(opts...))
}
