package vivid

import (
	"time"
)

type ClusterMemberInfo struct {
	Address string
	Version string
}

type ClusterContext interface {
	// Name 返回当前集群的名称。
	Name() string

	// GetMembers 获取当前集群中的所有节点。
	//
	// 返回值：
	//   - []ActorRef：当前集群中的所有节点。
	//   - error：获取节点失败时的错误。
	GetMembers() ([]ClusterMemberInfo, error)

	// Leader 返回当前集群选主结果中的 Leader 节点引用；若无成员或未达多数派可能为空。
	// 调用方应结合 InQuorum 判断是否可信任当前 Leader（分区时少数派不应以 Leader 做关键决策）。
	Leader() (ActorRef, error)

	// IsLeader 返回当前节点是否为当前视图下的 Leader。
	IsLeader() (bool, error)

	// InQuorum 返回当前节点是否处于多数派（未分区或非少数派）。
	// 为 false 时不应以 Leader 做关键决策（如单例迁移、写仲裁等）。
	InQuorum() (bool, error)

	// SetNodeVersion 设置本节点在集群中对外展示的版本号，会体现在成员信息中。
	SetNodeVersion(version string)

	// UpdateMembers 将外部服务发现得到的节点地址列表推送给集群，与本地视图合并。
	// 典型用法：从 etcd/consul 等拉取到节点列表后调用，与种子发现可并存。
	UpdateMembers(addresses []string)

	// Leave 让本节点主动离开集群：向已知成员广播离场并从本地成员表移除自身，
	// 其它节点收到后会立即移除本节点，适用于优雅下线。
	Leave()
}

// ClusterOptions 封装集群节点（NodeActor）的启动期配置，所有字段均在创建时确定，设计为不可变、不在运行时修改。
//
// 通过 NewClusterOptions 与一系列 ClusterOption 函数构建；未显式设置的项将使用默认值（见各 With* 函数及常量 defaultCluster*）。
// 构建后的 *ClusterOptions 会传入 SpawnNodeActor，并由 NodeActor 持有并只读使用。
type ClusterOptions struct {
	// ClusterName 本节点所属集群的逻辑名称。仅与 ClusterName 相同的节点会交换成员视图并视为同一集群；
	// 空串表示不校验集群名，与任意节点互通。用于多集群同网时的逻辑隔离。
	ClusterName string
	// Seeds 发现阶段使用的种子节点地址列表，格式为 "host:port"（与 Remoting 广告地址一致）。
	// 节点启动后会向 Seeds 以及已发现的成员周期性发送 GetNodesRequest，以收敛成员视图。建议至少配置 2 个以上种子以保证高可用。
	Seeds []string
	// DiscoveryInterval 两次发现轮次之间的间隔。每轮会向种子与部分成员发送请求并合并响应。默认 10s。
	DiscoveryInterval time.Duration
	// FailureDetectionTimeout 故障检测超时时长：若某成员在此时长内未被任何「直接响应」刷新 LastSeen，将从本地视图中剔除并发布成员变更事件。
	// 设为 ≤0 表示不自动剔除，仅依赖显式 Leave。默认 40s。
	FailureDetectionTimeout time.Duration
	// MaxDiscoveryTargetsPerTick 每轮发现中最多向多少个目标发送 GetNodesRequest；种子始终全部包含，其余从成员中按 LastProbed 优先选取。
	// ≤0 表示不限制，每轮向所有种子与成员发送。默认 20，用于控制每轮请求量。
	MaxDiscoveryTargetsPerTick int
}

// ClusterOption 是用于配置 ClusterOptions 的函数类型，采用函数式 Option 模式，便于链式/组合配置并保持向后兼容。
type ClusterOption = func(*ClusterOptions)

// NewClusterOptions 根据默认值与传入的 Option 列表构造 *ClusterOptions。
//
// 行为与 NewActorSystemOptions 一致：先将默认 Option 置于列表首部，再创建空 ClusterOptions，最后按顺序应用列表中每个 Option；
// 因此用户传入的 Option 会覆盖同名字段的默认值。调用方通常将结果传给 SpawnNodeActor 或 internal/cluster.SpawnNodeActorWithOptions。
func NewClusterOptions(opts ...ClusterOption) *ClusterOptions {
	opts = append([]ClusterOption{
		WithClusterDiscoveryInterval(10 * time.Second),
		WithClusterFailureDetectionTimeout(40 * time.Second),
		WithClusterMaxDiscoveryTargetsPerTick(20),
	}, opts...)

	o := &ClusterOptions{}
	for _, f := range opts {
		f(o)
	}
	return o
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
