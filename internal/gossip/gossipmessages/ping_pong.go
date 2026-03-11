// Package gossipmessages 定义 gossip 协议中的 Ping/Pong 消息结构及构造方法。
package gossipmessages

import (
	"time"

	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/internal/gossip/memberlist"
	"github.com/kercylan98/vivid/internal/gossip/versionvector"
)

// NewPing 构造一条 Ping 消息，携带发送方信息、其当前成员列表与版本向量，供对端合并与回 Pong。
func NewPing(info *endpoint.Information, memberList *memberlist.MemberList, versionVector *versionvector.VersionVector) *Ping {
	return &Ping{
		Timestamp:  time.Now(),
		Info:       info,
		MemberList: memberList,
		Version:    versionVector,
	}
}

// Ping 表示 gossip 请求：发送方用其视图（Info + MemberList + Version）向对端交换并请求对端视图。
type Ping struct {
	Timestamp  time.Time                    // 发送时间，可用于延迟或调试
	Info       *endpoint.Information        // 发送方端点信息
	MemberList *memberlist.MemberList       // 发送方当前成员列表
	Version    *versionvector.VersionVector // 发送方版本向量，用于因果比较与合并
}

// NewPong 构造一条 Pong 响应，携带本节点信息与当前视图，供请求方合并与判断是否已加入。
func NewPong(info *endpoint.Information, memberList *memberlist.MemberList, versionVector *versionvector.VersionVector) *Pong {
	return &Pong{
		Timestamp:  time.Now(),
		Info:       info,
		MemberList: memberList,
		Version:    versionVector,
	}
}

// Pong 表示 gossip 响应：回传本节点信息与视图，供请求方合并并可能触发状态迁移（如 Joining -> Up）。
type Pong struct {
	Timestamp  time.Time
	Info       *endpoint.Information
	MemberList *memberlist.MemberList
	Version    *versionvector.VersionVector
}
