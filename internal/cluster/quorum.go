package cluster

import (
	"sort"

	"github.com/kercylan98/vivid"
)

// QuorumCalculator 根据视图与配置计算是否满足法定人数。
type QuorumCalculator struct {
	options vivid.ClusterOptions
}

// NewQuorumCalculator 根据集群配置创建法定人数计算器。
func NewQuorumCalculator(options vivid.ClusterOptions) *QuorumCalculator {
	return &QuorumCalculator{options: options}
}

// SatisfiesQuorum 判断当前视图是否满足配置的法定人数策略。
func (q *QuorumCalculator) SatisfiesQuorum(v *ClusterView) bool {
	if v == nil || v.Members == nil {
		return false
	}
	var base bool
	switch q.options.QuorumStrategy {
	case vivid.QuorumStrategyMajorityDCs:
		dcTotal := make(map[string]int)
		dcHealthy := make(map[string]int)
		for _, m := range v.Members {
			if m == nil {
				continue
			}
			dc := m.Datacenter()
			if dc == "" {
				dc = "_default"
			}
			dcTotal[dc]++
			if m.Status == MemberStatusUp {
				dcHealthy[dc]++
			}
		}
		totalDCs := len(dcTotal)
		if totalDCs == 0 {
			base = false
			break
		}
		participating := 0
		for dc := range dcTotal {
			if dcHealthy[dc] >= 1 {
				participating++
			}
		}
		required := (totalDCs + 1) / 2
		base = participating >= required
	case vivid.QuorumStrategyAtLeastOnePerDC:
		dcHealthy := make(map[string]int)
		dcTotal := make(map[string]int)
		for _, m := range v.Members {
			if m == nil {
				continue
			}
			dc := m.Datacenter()
			if dc == "" {
				dc = "_default"
			}
			dcTotal[dc]++
			if m.Status == MemberStatusUp {
				dcHealthy[dc]++
			}
		}
		base = len(dcTotal) > 0
		for dc, n := range dcTotal {
			if n > 0 && dcHealthy[dc] < 1 {
				base = false
				break
			}
		}
	default:
		base = v.QuorumSize > 0 && v.HealthyCount >= v.QuorumSize
	}
	if !base {
		return false
	}
	if len(q.options.RequiredDCsForQuorum) > 0 {
		dcHealthy := make(map[string]int)
		for _, m := range v.Members {
			if m == nil {
				continue
			}
			dc := m.Datacenter()
			if dc == "" {
				dc = "_default"
			}
			if m.Status == MemberStatusUp {
				dcHealthy[dc]++
			}
		}
		for _, dc := range q.options.RequiredDCsForQuorum {
			d := dc
			if d == "" {
				d = "_default"
			}
			if dcHealthy[d] < 1 {
				return false
			}
		}
	}
	return true
}

// ComputeLeaderAddr 按当前视图做确定性选主：取状态为 Up 的成员按 Address 排序后的首个地址。
func ComputeLeaderAddr(v *ClusterView) string {
	if v == nil || len(v.Members) == 0 {
		return ""
	}
	var addresses []string
	for _, m := range v.Members {
		if m != nil && m.Status == MemberStatusUp && m.Address != "" {
			addresses = append(addresses, m.Address)
		}
	}
	if len(addresses) == 0 {
		return ""
	}
	sort.Strings(addresses)
	return addresses[0]
}
