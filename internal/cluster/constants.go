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

const (
	DefaultGossipInterval       = 1 * time.Second
	DefaultMaxTargetsPerTick    = 20
	InitialJoinRetryDelay       = 2 * time.Second
	MaxJoinRetryDelay           = 30 * time.Second
	MaxGetViewTargets           = 5
	LeaveGraceTimeout           = 5 * time.Second
	MaxJoinRateLimitEntries     = 10000
)
