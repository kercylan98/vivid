package gossip

import (
	"math/rand/v2"
	"slices"
	"time"

	"github.com/kercylan98/vivid"
)

type Option func(*Options)

func NewOptions(options ...Option) *Options {
	opts := &Options{
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

type Options struct {
	Seeds                   []vivid.ActorRef // 加入集群时使用的种子节点 ActorRef 列表
	LaunchDelay             time.Duration    // 延迟启动时间，避免所有节点同时启动，导致集群瞬间压力过大，通常以随机值设置，默认 50~1000ms
	GossipInterval          time.Duration    // 周期向集群内节点交换视图 gossip 的间隔，默认 200 ms
	GossipPeersCount        int              // 周期向集群内节点交换视图 gossip 时，向每个节点发送 gossip 的数量，默认 5
	GracefulShutdownTimeout time.Duration    // 优雅退出超时时间，默认 10 秒
	JoinTimeout             time.Duration    // 加入集群时，向种子节点发送 Ping 的超时时间，默认 3 秒
}

func WithOptions(options *Options) Option {
	return func(opts *Options) {
		if options == nil {
			return
		}

		*opts = *options
		var newSeeds []vivid.ActorRef
		for _, seed := range opts.Seeds {
			if seed == nil {
				continue
			}
			newSeeds = append(newSeeds, seed)
		}
		opts.Seeds = newSeeds
	}
}

func WithSeeds(seeds ...vivid.ActorRef) Option {
	return func(options *Options) {
		options.Seeds = append(options.Seeds, slices.Clone(seeds)...)
		var newSeeds []vivid.ActorRef
		for _, seed := range options.Seeds {
			if seed == nil {
				continue
			}
			newSeeds = append(newSeeds, seed)
		}
		options.Seeds = newSeeds
	}
}

func WithLaunchDelay(delay time.Duration) Option {
	return func(options *Options) {
		if delay < 0 {
			return
		}
		options.LaunchDelay = delay
	}
}

func WithGossipInterval(interval time.Duration) Option {
	return func(options *Options) {
		if interval <= 0 {
			return
		}
		options.GossipInterval = interval
	}
}

func WithGossipPeersCount(count int) Option {
	return func(options *Options) {
		if count <= 0 {
			return
		}
		options.GossipPeersCount = count
	}
}

func WithGracefulShutdownTimeout(timeout time.Duration) Option {
	return func(options *Options) {
		if timeout < 0 {
			return
		}
		options.GracefulShutdownTimeout = timeout
	}
}

func WithJoinTimeout(timeout time.Duration) Option {
	return func(options *Options) {
		if timeout <= 0 {
			return
		}
		options.JoinTimeout = timeout
	}
}
