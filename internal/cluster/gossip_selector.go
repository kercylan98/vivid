package cluster

import (
	"math/rand"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/utils"
)

// GetAllSeedsWithDCFunc 返回所有种子地址及地址到 DC 的映射。
type GetAllSeedsWithDCFunc func() (Addresses []string, dcByAddr map[string]string)

// GossipTargetSelector 负责选择每轮 Gossip 的目标节点（同 DC/同 Region 优先）。
type GossipTargetSelector struct {
	options        vivid.ClusterOptions
	getSeedsWithDC GetAllSeedsWithDCFunc
}

// NewGossipTargetSelector 创建 Gossip 目标选择器；getSeedsWithDC 通常由 SeedsProvider.GetAllSeedsWithDC 提供。
func NewGossipTargetSelector(options vivid.ClusterOptions, getSeedsWithDC GetAllSeedsWithDCFunc) *GossipTargetSelector {
	return &GossipTargetSelector{
		options:        options,
		getSeedsWithDC: getSeedsWithDC,
	}
}

// SelectTargets 返回本轮 Gossip 的目标地址列表，同 DC/同 Region 优先，数量受 MaxDiscoveryTargetsPerTick 限制。
func (g *GossipTargetSelector) SelectTargets(v *ClusterView, nodeState *NodeState) []string {
	if v == nil {
		return nil
	}
	tick := g.options.MaxDiscoveryTargetsPerTick
	if tick <= 0 {
		tick = DefaultMaxTargetsPerTick
	}
	self, _ := utils.NormalizeAddress(nodeState.Address)
	selfDC := nodeState.Datacenter()
	selfRegion := nodeState.Region()
	seen := make(map[string]bool)
	seedAddresses, seedDC := g.getSeedsWithDC()
	if selfRegion == "" {
		var sameDC, otherDC []string
		for _, s := range seedAddresses {
			if s == self || seen[s] {
				continue
			}
			seen[s] = true
			if selfDC != "" && seedDC[s] != "" && seedDC[s] != selfDC {
				otherDC = append(otherDC, s)
			} else {
				sameDC = append(sameDC, s)
			}
		}
		for _, m := range v.Members {
			addr, ok := utils.NormalizeAddress(m.Address)
			if !ok || addr == self || seen[addr] {
				continue
			}
			seen[addr] = true
			if selfDC != "" && m.Datacenter() != selfDC {
				otherDC = append(otherDC, addr)
			} else {
				sameDC = append(sameDC, addr)
			}
		}
		rand.Shuffle(len(sameDC), func(i, j int) { sameDC[i], sameDC[j] = sameDC[j], sameDC[i] })
		rand.Shuffle(len(otherDC), func(i, j int) { otherDC[i], otherDC[j] = otherDC[j], otherDC[i] })
		candidates := append(sameDC, otherDC...)
		if len(candidates) <= tick {
			return candidates
		}
		return candidates[:tick]
	}
	var sameRegSameDC, sameRegOtherDC, otherRegSameDC, otherRegOtherDC []string
	for _, s := range seedAddresses {
		if s == self || seen[s] {
			continue
		}
		seen[s] = true
		if selfDC != "" && seedDC[s] != "" && seedDC[s] != selfDC {
			otherRegOtherDC = append(otherRegOtherDC, s)
		} else {
			sameRegSameDC = append(sameRegSameDC, s)
		}
	}
	for _, m := range v.Members {
		addr, ok := utils.NormalizeAddress(m.Address)
		if !ok || addr == self || seen[addr] {
			continue
		}
		seen[addr] = true
		sameReg := m.Region() != "" && m.Region() == selfRegion
		sameDC := selfDC == "" || m.Datacenter() == selfDC
		if sameReg && sameDC {
			sameRegSameDC = append(sameRegSameDC, addr)
		} else if sameReg {
			sameRegOtherDC = append(sameRegOtherDC, addr)
		} else if sameDC {
			otherRegSameDC = append(otherRegSameDC, addr)
		} else {
			otherRegOtherDC = append(otherRegOtherDC, addr)
		}
	}
	rand.Shuffle(len(sameRegSameDC), func(i, j int) { sameRegSameDC[i], sameRegSameDC[j] = sameRegSameDC[j], sameRegSameDC[i] })
	rand.Shuffle(len(sameRegOtherDC), func(i, j int) { sameRegOtherDC[i], sameRegOtherDC[j] = sameRegOtherDC[j], sameRegOtherDC[i] })
	rand.Shuffle(len(otherRegSameDC), func(i, j int) { otherRegSameDC[i], otherRegSameDC[j] = otherRegSameDC[j], otherRegSameDC[i] })
	rand.Shuffle(len(otherRegOtherDC), func(i, j int) { otherRegOtherDC[i], otherRegOtherDC[j] = otherRegOtherDC[j], otherRegOtherDC[i] })
	candidates := append(append(append(sameRegSameDC, sameRegOtherDC...), otherRegSameDC...), otherRegOtherDC...)
	if len(candidates) <= tick {
		return candidates
	}
	return candidates[:tick]
}

// SelectTargetsCrossDC 仅返回跨 DC 目标，用于 CrossDCDiscoveryInterval 的单独轮次。
func (g *GossipTargetSelector) SelectTargetsCrossDC(v *ClusterView, nodeState *NodeState) []string {
	if v == nil {
		return nil
	}
	dc := g.options.MaxDiscoveryTargetsPerTickCrossDC
	if dc <= 0 {
		dc = g.options.MaxDiscoveryTargetsPerTick
	}
	if dc <= 0 {
		dc = DefaultMaxTargetsPerTick
	}
	self, _ := utils.NormalizeAddress(nodeState.Address)
	selfDC := nodeState.Datacenter()
	if selfDC == "" {
		return nil
	}
	var otherDC []string
	seedAddresses, seedDC := g.getSeedsWithDC()
	for _, s := range seedAddresses {
		if s == self {
			continue
		}
		if seedDC[s] != "" && seedDC[s] != selfDC {
			otherDC = append(otherDC, s)
		}
	}
	seen := make(map[string]bool)
	for _, addr := range otherDC {
		seen[addr] = true
	}
	for _, m := range v.Members {
		addr, ok := utils.NormalizeAddress(m.Address)
		if !ok || addr == self || seen[addr] {
			continue
		}
		if m.Datacenter() != selfDC {
			seen[addr] = true
			otherDC = append(otherDC, addr)
		}
	}
	if len(otherDC) <= dc {
		return otherDC
	}
	rand.Shuffle(len(otherDC), func(i, j int) { otherDC[i], otherDC[j] = otherDC[j], otherDC[i] })
	return otherDC[:dc]
}
