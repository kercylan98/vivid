package cluster

import (
	"math/rand"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/utils"
)

// SeedsProvider 管理种子地址解析：按 DC 的种子列表、加入用种子顺序等。
type SeedsProvider struct {
	options vivid.ClusterOptions
}

// NewSeedsProvider 根据集群配置创建种子提供者。
func NewSeedsProvider(options vivid.ClusterOptions) *SeedsProvider {
	return &SeedsProvider{options: options}
}

// GetAllSeedsWithDC 返回所有种子地址及来自 SeedsByDC 的地址对应的 DC；若配置了 SeedsResolver 则使用其返回值。
func (s *SeedsProvider) GetAllSeedsWithDC() (addrs []string, dcByAddr map[string]string) {
	dcByAddr = make(map[string]string)
	seen := make(map[string]bool)
	seedsByDC := s.options.SeedsByDC
	seeds := s.options.Seeds
	if s.options.SeedsResolver != nil {
		seedsByDC = s.options.SeedsResolver.GetSeedsByDC()
		seeds = s.options.SeedsResolver.GetSeeds()
	}
	for dc, list := range seedsByDC {
		for _, addr := range utils.NormalizeAddresses(list) {
			if !seen[addr] {
				seen[addr] = true
				addrs = append(addrs, addr)
			}
			dcByAddr[addr] = dc
		}
	}
	for _, addr := range utils.NormalizeAddresses(seeds) {
		if !seen[addr] {
			seen[addr] = true
			addrs = append(addrs, addr)
		}
	}
	return addrs, dcByAddr
}

// GetSeedsForJoin 返回用于加入的种子列表，本 DC 种子优先；若配置了 SeedsResolver 则使用其返回值。
func (s *SeedsProvider) GetSeedsForJoin(selfDC string) []string {
	seedsByDC := s.options.SeedsByDC
	seeds := s.options.Seeds
	if s.options.SeedsResolver != nil {
		seedsByDC = s.options.SeedsResolver.GetSeedsByDC()
		seeds = s.options.SeedsResolver.GetSeeds()
	}
	if len(seedsByDC) > 0 {
		var sameDC, otherDC []string
		seen := make(map[string]bool)
		if selfDC != "" && len(seedsByDC[selfDC]) > 0 {
			for _, addr := range utils.NormalizeAddresses(seedsByDC[selfDC]) {
				if !seen[addr] {
					seen[addr] = true
					sameDC = append(sameDC, addr)
				}
			}
		}
		for dc, addrs := range seedsByDC {
			if dc == selfDC {
				continue
			}
			for _, addr := range utils.NormalizeAddresses(addrs) {
				if !seen[addr] {
					seen[addr] = true
					otherDC = append(otherDC, addr)
				}
			}
		}
		for _, addr := range utils.NormalizeAddresses(seeds) {
			if !seen[addr] {
				seen[addr] = true
				otherDC = append(otherDC, addr)
			}
		}
		rand.Shuffle(len(sameDC), func(i, j int) { sameDC[i], sameDC[j] = sameDC[j], sameDC[i] })
		rand.Shuffle(len(otherDC), func(i, j int) { otherDC[i], otherDC[j] = otherDC[j], otherDC[i] })
		return append(sameDC, otherDC...)
	}
	return utils.NormalizeAddresses(seeds)
}
