package cluster

import "time"

// 调度器引用，用于 Cancel/Loop 标识
const (
	SchedRefGossip           = "cluster-gossip"
	SchedRefGossipCrossDC    = "cluster-gossip-cross-dc"
	SchedRefFailureDetection = "cluster-failure-detection"
	SchedRefJoinRetry        = "cluster-join-retry"
	SchedRefLeaveDelay       = "cluster-leave-delay"
)

// ClusterSingletonsPathPrefix 集群单例 Manager 及其子 Actor 的路径前缀，用于 SingletonRef 解析。
const ClusterSingletonsPathPrefix = "/@cluster-singletons"

const (
	DefaultGossipInterval    = 1 * time.Second
	DefaultMaxTargetsPerTick = 20
	InitialJoinRetryDelay    = 2 * time.Second
	MaxJoinRetryDelay        = 30 * time.Second
	MaxGetViewTargets        = 5
	MaxJoinRateLimitEntries  = 10000
)
