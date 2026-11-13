// Package gossipmessages 定义 gossip 协议中的 Ping/Pong 消息结构及构造方法。
package gossipmessages

import (
	"time"

	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/internal/gossip/memberlist"
	"github.com/kercylan98/vivid/internal/gossip/versionvector"
)

const (
	GossipTypeJoin                string = "JOIN_CLUSTER"          // 加入集群
	GossipTypeCycleSYNC           string = "CYCLE_SYNC"            // 周期同步
	GossipTypeDiscoverNewNode     string = "DISCOVER_NEW_NODE"     // 发现新节点
	GossipTypeRemoveNode          string = "REMOVE_NODE"           // 移除节点
	GossipTypeCriticalStateChange string = "CRITICAL_STATE_CHANGE" // 关键状态变更
)

// NewPing 构造一条 Ping 消息，携带发送方信息、其当前成员列表与版本向量，供对端合并与回 Pong。
func NewPing(clusterName, gossipType string, info *endpoint.Information, memberList *memberlist.MemberList, versionVector *versionvector.VersionVector) *Ping {
	return &Ping{
		ClusterName: clusterName,
		GossipType:  gossipType,
		Timestamp:   time.Now(),
		Info:        info,
		MemberList:  memberList,
		Version:     versionVector,
	}
}

// Ping 表示 gossip 请求：发送方用其视图（Info + MemberList + Version）向对端交换并请求对端视图。
type Ping struct {
	ClusterName   string                       // 逻辑集群名称，合并视图前必须完全匹配
	ProbeSequence uint64                       // 直接往返请求序号，0 表示仅交换视图
	GossipType    string                       //  gossip 类型，用于区分不同的 gossip 消息
	Timestamp     time.Time                    // 发送时间，可用于延迟或调试
	Info          *endpoint.Information        // 发送方端点信息
	MemberList    *memberlist.MemberList       // 发送方当前成员列表
	Version       *versionvector.VersionVector // 发送方版本向量，用于因果比较与合并
}

// NewPong 构造一条 Pong 响应，携带本节点信息与当前视图，供请求方合并与判断是否已加入。
func NewPong(clusterName, gossipType string, info *endpoint.Information, memberList *memberlist.MemberList, versionVector *versionvector.VersionVector) *Pong {
	return &Pong{
		ClusterName: clusterName,
		GossipType:  gossipType,
		Timestamp:   time.Now(),
		Info:        info,
		MemberList:  memberList,
		Version:     versionVector,
	}
}

// Pong 表示 gossip 响应：回传本节点信息与视图，供请求方合并并可能触发状态迁移（如 Joining -> Up）。
type Pong struct {
	ClusterName   string // 逻辑集群名称，合并视图前必须完全匹配
	ProbeSequence uint64 // 回显直接往返请求序号
	ProbeOriginID string // 发起观察的实例 ID，隔离同地址重启前的请求
	GossipType    string //  gossip 类型，用于区分不同的 gossip 消息
	Timestamp     time.Time
	Info          *endpoint.Information
	MemberList    *memberlist.MemberList
	Version       *versionvector.VersionVector
}

// JoinRejected 仅在集群名称不匹配时返回，不携带成员视图或加入确认。
type JoinRejected struct {
	ClusterName string // 接收节点配置的集群名称
}
