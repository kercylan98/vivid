package vivid

import (
	"math/rand/v2"
	"net"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

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
		ProbeTimeout:            3 * time.Second,
		ReachabilityTTL:         10 * time.Second,
	}
	for _, option := range options {
		option(opts)
	}
	return opts
}

type ActorSystemClusterOptions struct {
	Name                    string        // 逻辑集群名称，必填，区分大小写，最多 128 字节
	Seeds                   []string      // 集群种子节点地址列表
	Bootstrap               bool          // 配置了种子时仍主动独立启动，默认 false
	LaunchDelay             time.Duration // 延迟启动时间，避免所有节点同时启动，导致集群瞬间压力过大，通常以随机值设置，默认 50~1000ms
	GossipInterval          time.Duration // 周期向集群内节点交换视图 gossip 的间隔，默认 200 ms
	GossipPeersCount        int           // 每轮 Gossip 最多联络的目标地址数量，默认 5
	GracefulShutdownTimeout time.Duration // 优雅退出超时时间，默认 10 秒
	JoinTimeout             time.Duration // 每个种子的初始加入总超时，包含有限重发，默认 3 秒
	ProbeTimeout            time.Duration // Gossip 直接往返的等待预算，默认 3 秒
	ReachabilityTTL         time.Duration // 可达性观察结果的有效期，默认 10 秒
}

// Validate 校验集群参数，不修改调用方配置，也不解析 DNS。
func (o *ActorSystemClusterOptions) Validate() error {
	if o == nil {
		return ErrorIllegalArgument.WithMessage("cluster options are nil")
	}
	if len(o.Name) == 0 || len(o.Name) > 128 || !utf8.ValidString(o.Name) || strings.ContainsFunc(o.Name, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return ErrorIllegalArgument.WithMessage("cluster name must be valid UTF-8, 1 to 128 bytes, without whitespace or control characters")
	}
	if o.LaunchDelay < 0 || o.GossipInterval <= 0 || o.GossipPeersCount <= 0 || o.GracefulShutdownTimeout <= 0 || o.JoinTimeout <= 0 || o.ProbeTimeout <= 0 || o.ReachabilityTTL <= 0 {
		return ErrorIllegalArgument.WithMessage("cluster launch delay must be non-negative; gossip interval, peers count, shutdown timeout, join timeout, probe timeout and reachability TTL must be positive")
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

// WithActorSystemClusterName 设置必填的逻辑集群名称；只有名称完全相同的节点才交换成员视图。
// 名称不作为认证凭据，不能替代 TLS 或业务授权。
func WithActorSystemClusterName(name string) ActorSystemClusterOption {
	return func(o *ActorSystemClusterOptions) { o.Name = name }
}

// WithActorSystemClusterProbeTimeout 设置一次 Gossip 直接往返的总等待预算，重复发送不延长预算。
func WithActorSystemClusterProbeTimeout(timeout time.Duration) ActorSystemClusterOption {
	return func(o *ActorSystemClusterOptions) { o.ProbeTimeout = timeout }
}

// WithActorSystemClusterReachabilityTTL 设置本地可达性观察的有效期，过期后回到 Unknown。
func WithActorSystemClusterReachabilityTTL(ttl time.Duration) ActorSystemClusterOption {
	return func(o *ActorSystemClusterOptions) { o.ReachabilityTTL = ttl }
}

// WithActorSystemClusterBootstrap 显式允许独立启动，并在 Up 后继续联络配置的种子。
func WithActorSystemClusterBootstrap(bootstrap bool) ActorSystemClusterOption {
	return func(o *ActorSystemClusterOptions) { o.Bootstrap = bootstrap }
}

// WithActorSystemClusterJoinTimeout 设置每个去重后的非本地种子的加入总超时，包含有限重发。
func WithActorSystemClusterJoinTimeout(timeout time.Duration) ActorSystemClusterOption {
	return func(o *ActorSystemClusterOptions) { o.JoinTimeout = timeout }
}

// WithActorSystemClusterOptions 返回一个 ActorSystemOption，用于启用并配置 ActorSystem 的集群。
// 首次调用创建默认选项；后续调用依次修改已有配置，不重置未指定的字段。
// 未传入子选项时也会启用集群，启动前仍需明确设置名称并启用 Remoting。
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
