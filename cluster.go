package vivid

import (
	"math/rand/v2"
	"time"
)

type ActorSystemClusterOption func(options *ActorSystemClusterOptions)

func NewActorSystemClusterOptions(options ...ActorSystemClusterOption) *ActorSystemClusterOptions {
	opts := &ActorSystemClusterOptions{
		LaunchDelay:             time.Duration(rand.IntN(950)+50) * time.Millisecond,
		GossipInterval:          200 * time.Millisecond,
		GossipPeersCount:        5,
		GracefulShutdownTimeout: 10 * time.Second,
		JoinTimeout:             3 * time.Second,
	}
	for _, option := range options {
		option(opts)
	}
	return opts
}

type ActorSystemClusterOptions struct {
	Seeds                   []string      // 集群种子节点地址列表
	LaunchDelay             time.Duration // 延迟启动时间，避免所有节点同时启动，导致集群瞬间压力过大，通常以随机值设置，默认 50~1000ms
	GossipInterval          time.Duration // 周期向集群内节点交换视图 gossip 的间隔，默认 200 ms
	GossipPeersCount        int           // 周期向集群内节点交换视图 gossip 时，向每个节点发送 gossip 的数量，默认 5
	GracefulShutdownTimeout time.Duration // 优雅退出超时时间，默认 10 秒
	JoinTimeout             time.Duration // 加入集群时，向种子节点发送 Ping 的超时时间，默认 3 秒
}

// WithActorSystemClusterOptions 返回一个 ActorSystemOption，用于配置 ActorSystem 的集群选项。
//
// 参数：
//   - options: 期望设置的集群选项。
//   - opts ...ActorSystemClusterOption: 可选参数，链式扩展集群选项。
//
// 返回值：
//   - ActorSystemOption: 可传给 NewActorSystem 或其它配置参数的 Option 函数。
func WithActorSystemClusterOptions(options ActorSystemClusterOptions, opts ...ActorSystemClusterOption) ActorSystemOption {
	return func(o *ActorSystemOptions) {
		o.ClusterOptions = &options
		for _, opt := range opts {
			opt(o.ClusterOptions)
		}
	}
}

// WithActorSystemClusterSeeds 返回一个 ActorSystemClusterOption，用于配置 ActorSystem 的集群种子节点地址列表。
//
// 参数：
//   - seeds: 集群种子节点地址列表。
//
// 返回值：
//   - ActorSystemClusterOption: 可传给 NewActorSystemClusterOptions 或其它配置参数的 Option 函数。
func WithActorSystemClusterSeeds(seeds ...string) ActorSystemClusterOption {
	return func(o *ActorSystemClusterOptions) {
		o.Seeds = append(o.Seeds, seeds...)
	}
}

func WithActorSystemClusterLaunchDelay(delay time.Duration) ActorSystemClusterOption {
	return func(o *ActorSystemClusterOptions) {
		o.LaunchDelay = delay
	}
}

func WithActorSystemClusterGossipInterval(interval time.Duration) ActorSystemClusterOption {
	return func(o *ActorSystemClusterOptions) {
		o.GossipInterval = interval
	}
}

func WithActorSystemClusterGossipPeersCount(count int) ActorSystemClusterOption {
	return func(o *ActorSystemClusterOptions) {
		o.GossipPeersCount = count
	}
}

func WithActorSystemClusterGracefulShutdownTimeout(timeout time.Duration) ActorSystemClusterOption {
	return func(o *ActorSystemClusterOptions) {
		o.GracefulShutdownTimeout = timeout
	}
}
