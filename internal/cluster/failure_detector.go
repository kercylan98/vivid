package cluster

import (
	"time"

	"github.com/kercylan98/vivid"
)

// FailureDetector 根据 LastSeen 与超时配置判断成员 Suspect/Down，输出待标记与待移除列表。
type FailureDetector struct {
	options vivid.ClusterOptions
}

// NewFailureDetector 根据集群配置创建故障检测器。
func NewFailureDetector(options vivid.ClusterOptions) *FailureDetector {
	return &FailureDetector{options: options}
}

// TimeoutFor 返回对某成员的故障检测超时（同 DC 与跨 DC 可不同）。
func (f *FailureDetector) TimeoutFor(member *NodeState, selfDC string) time.Duration {
	t := f.options.FailureDetectionTimeout
	if t <= 0 {
		return 0
	}
	if selfDC != "" && member != nil && member.Datacenter() != "" && member.Datacenter() != selfDC {
		if f.options.CrossDCFailureDetectionTimeout > 0 {
			return f.options.CrossDCFailureDetectionTimeout
		}
		return t * 2
	}
	return t
}

// RunDetection 根据当前视图与时间运行一轮检测，返回应标记为 Suspect 与应移除的节点 ID 列表。
func (f *FailureDetector) RunDetection(v *ClusterView, selfAddr string, selfDC string, now time.Time) (toSuspect, toRemove []string) {
	if v == nil {
		return nil, nil
	}
	confirmDur := f.options.SuspectConfirmDuration
	if confirmDur < 0 {
		confirmDur = 0
	}
	for id, m := range v.Members {
		if m == nil || m.Address == selfAddr {
			continue
		}
		suspectTimeout := f.TimeoutFor(m, selfDC)
		if suspectTimeout <= 0 {
			continue
		}
		downTimeout := suspectTimeout + confirmDur
		suspectThreshold := now.Add(-suspectTimeout).UnixNano()
		downThreshold := now.Add(-downTimeout).UnixNano()
		if m.LastSeen < downThreshold {
			toRemove = append(toRemove, id)
		} else if m.Status == MemberStatusUp && m.LastSeen < suspectThreshold && confirmDur > 0 {
			toSuspect = append(toSuspect, id)
		} else if m.Status == MemberStatusSuspect && m.LastSeen < downThreshold {
			toRemove = append(toRemove, id)
		}
	}
	return toSuspect, toRemove
}
