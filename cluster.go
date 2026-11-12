package vivid

import (
	"math/rand/v2"
	"net"
	"strings"
	"time"

	"github.com/kercylan98/vivid/internal/utils"
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
	Bootstrap               bool          // 配置了种子时仍主动独立启动，默认 false
	LaunchDelay             time.Duration // 延迟启动时间，避免所有节点同时启动，导致集群瞬间压力过大，通常以随机值设置，默认 50~1000ms
	GossipInterval          time.Duration // 周期向集群内节点交换视图 gossip 的间隔，默认 200 ms
	GossipPeersCount        int           // 周期向集群内节点交换视图 gossip 时，向每个节点发送 gossip 的数量，默认 5
	GracefulShutdownTimeout time.Duration // 优雅退出超时时间，默认 10 秒
	JoinTimeout             time.Duration // 每个种子的初始加入总超时，包含有限重发，默认 3 秒
}

// Validate 校验集群参数，不修改调用方配置，也不解析 DNS。
func (o *ActorSystemClusterOptions) Validate() error {
	if o == nil {
		return ErrorIllegalArgument.WithMessage("cluster options are nil")
	}
	if o.LaunchDelay < 0 || o.GossipInterval <= 0 || o.GossipPeersCount <= 0 || o.GracefulShutdownTimeout <= 0 || o.JoinTimeout <= 0 {
		return ErrorIllegalArgument.WithMessage("cluster launch delay must be non-negative; gossip interval, peers count, shutdown timeout and join timeout must be positive")
	}
	for _, seed := range o.Seeds {
		address, ok := utils.NormalizeAddress(seed)
		host, _, err := net.SplitHostPort(address)
		if !ok || err != nil || strings.TrimSpace(host) == "" || host == "0.0.0.0" || host == "::" {
			return ErrorIllegalArgument.WithMessage("invalid cluster seed address: " + seed)
		}
	}
	return nil
}

// WithActorSystemClusterBootstrap 显式允许独立启动，并在 Up 后继续联络配置的种子。
func WithActorSystemClusterBootstrap(bootstrap bool) ActorSystemClusterOption {
	return func(o *ActorSystemClusterOptions) { o.Bootstrap = bootstrap }
}

// WithActorSystemClusterJoinTimeout 设置每个去重后的非本地种子的加入总超时，包含有限重发。
func WithActorSystemClusterJoinTimeout(timeout time.Duration) ActorSystemClusterOption {
	return func(o *ActorSystemClusterOptions) { o.JoinTimeout = timeout }
}

// WithActorSystemClusterOptions 返回一个 ActorSystemOption，用于配置 ActorSystem 的集群选项。
//
// 参数：
//   - opts ...ActorSystemClusterOption: 可选参数，链式扩展集群选项。
//
// 返回值：
//   - ActorSystemOption: 可传给 NewActorSystem 或其它配置参数的 Option 函数。
func WithActorSystemClusterOptions(opts ...ActorSystemClusterOption) ActorSystemOption {
	return func(o *ActorSystemOptions) {
		var options = o.ClusterOptions
		if options == nil {
			options = NewActorSystemClusterOptions()
		}
		for _, opt := range opts {
			opt(options)
		}
		o.ClusterOptions = options
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
