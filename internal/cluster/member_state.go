package cluster

import "time"

// memberState 用于本地成员表项，含最近可见时间供故障检测。
type memberState struct {
	Address    string
	Version    string
	LastSeen   time.Time
	LastProbed time.Time // 本节点上次向该成员发送 GetNodesRequest 的时间，用于优先选「未同步/最久未同步」的节点
}
