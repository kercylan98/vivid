package gossip

import (
	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/internal/gossip/memberlist"
	"github.com/kercylan98/vivid/internal/gossip/versionvector"
)

// outboundView 仅在 Actor 线程内构造，交给异步发送后不再修改或复用其存储。
// 成员或版本变化时丢弃缓存引用，已投递消息仍持有各自的旧视图。
type outboundView struct {
	info    *endpoint.Information
	members *memberlist.MemberList
	version *versionvector.VersionVector
}

func (a *Actor) currentOutboundView() *outboundView {
	if a.outboundView == nil {
		a.outboundView = &outboundView{
			info:    a.info.Clone(),
			members: a.view.Members().Clone(),
			version: a.view.Version().Clone(),
		}
	}
	return a.outboundView
}
