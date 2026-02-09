package cluster

import (
	"math"
	"time"

	"github.com/kercylan98/vivid"
)

// joinRateLimitEntry 用于按发送方地址的 Join 限流（token bucket）。
type joinRateLimitEntry struct {
	tokens     float64
	lastRefill time.Time
}

// JoinRateLimiter 按发送方地址做 Join 限流，管理 token bucket 状态。
type JoinRateLimiter struct {
	entries map[string]*joinRateLimitEntry
	rate    float64
	burst   float64
	maxEnt  int
}

// NewJoinRateLimiter 根据配置创建 Join 限流器；ratePerSec <= 0 表示不限流。
func NewJoinRateLimiter(ratePerSec int, burst int, maxEntries int) *JoinRateLimiter {
	if maxEntries <= 0 {
		maxEntries = MaxJoinRateLimitEntries
	}
	return &JoinRateLimiter{
		entries: make(map[string]*joinRateLimitEntry),
		rate:    float64(ratePerSec),
		burst:   float64(burst),
		maxEnt:  maxEntries,
	}
}

// Allow 按发送方地址做限流，返回 true 表示允许本次 Join。addr 为空时允许。
func (r *JoinRateLimiter) Allow(addr string) bool {
	if r.rate <= 0 || addr == "" {
		return true
	}
	if r.burst <= 0 {
		r.burst = 1
	}
	now := time.Now()
	ent := r.entries[addr]
	if ent == nil {
		if len(r.entries) >= r.maxEnt {
			var oldestAddr string
			oldestRefill := now
			for k, v := range r.entries {
				if v != nil && v.lastRefill.Before(oldestRefill) {
					oldestRefill = v.lastRefill
					oldestAddr = k
				}
			}
			if oldestAddr != "" {
				delete(r.entries, oldestAddr)
			}
		}
		ent = &joinRateLimitEntry{tokens: r.burst, lastRefill: now}
		r.entries[addr] = ent
	}
	elapsed := now.Sub(ent.lastRefill).Seconds()
	ent.tokens = math.Min(r.burst, ent.tokens+elapsed*r.rate)
	ent.lastRefill = now
	if ent.tokens >= 1 {
		ent.tokens--
		return true
	}
	return false
}

// GossipRateLimiter 全局 Gossip 发送限流（单桶）。
type GossipRateLimiter struct {
	entry *joinRateLimitEntry
	rate  float64
	burst float64
}

// NewGossipRateLimiter 根据配置创建 Gossip 限流器；ratePerSec <= 0 表示不限流。
func NewGossipRateLimiter(ratePerSec int, burst int) *GossipRateLimiter {
	if burst <= 0 {
		burst = 1
	}
	return &GossipRateLimiter{
		rate:  float64(ratePerSec),
		burst: float64(burst),
	}
}

// Allow 返回 true 表示允许发送一条 Gossip。
func (g *GossipRateLimiter) Allow() bool {
	if g.rate <= 0 {
		return true
	}
	now := time.Now()
	if g.entry == nil {
		g.entry = &joinRateLimitEntry{tokens: g.burst, lastRefill: now}
	}
	ent := g.entry
	elapsed := now.Sub(ent.lastRefill).Seconds()
	ent.tokens = math.Min(g.burst, ent.tokens+elapsed*g.rate)
	ent.lastRefill = now
	if ent.tokens >= 1 {
		ent.tokens--
		return true
	}
	return false
}

// ApplyJoinRateLimiterOptions 从 ClusterOptions 填充 JoinRateLimiter 的 rate/burst；用于 NodeActor 初始化。
func ApplyJoinRateLimiterOptions(opts vivid.ClusterOptions) (rate, burst, maxEnt int) {
	rate = int(opts.JoinRateLimitPerSecond)
	burst = opts.JoinRateLimitBurst
	if burst <= 0 {
		burst = 1
	}
	maxEnt = MaxJoinRateLimitEntries
	return rate, burst, maxEnt
}

// ApplyGossipRateLimiterOptions 从 ClusterOptions 填充 GossipRateLimiter 的 rate/burst。
func ApplyGossipRateLimiterOptions(opts vivid.ClusterOptions) (rate, burst int) {
	rate = int(opts.GossipRateLimitPerSecond)
	burst = opts.GossipRateLimitBurst
	if burst <= 0 {
		burst = 1
	}
	return rate, burst
}
