package gossip

import (
	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/internal/gossip/memberlist"
	"github.com/kercylan98/vivid/internal/gossip/versionvector"
)

// outboundView 仅在 Actor 线程内构造，交给异步发送后不再修改或复用成员与版本存储。
// 成员或版本变化时丢弃缓存引用，已投递消息仍持有各自的旧视图。
type outboundView struct {
	info    *endpoint.Information
	members *memberlist.MemberList
	version *versionvector.VersionVector
	indexes map[string]int
	seen    []uint64
	round   uint64
}

// matches 按成员身份比较请求已有的完整视图，不改变发送顺序。
// 索引和去重标记仅由 Actor 线程访问，不属于交给异步发送的成员与版本。
func (v *outboundView) matches(members *memberlist.MemberList, version *versionvector.VersionVector) bool {
	if len(members.List()) != len(v.members.List()) || !v.version.IsEqual(version) {
		return false
	}
	if v.indexes == nil {
		v.indexes = make(map[string]int, len(v.members.List()))
		v.seen = make([]uint64, len(v.members.List()))
		for index, member := range v.members.List() {
			v.indexes[member.ID()] = index
		}
	}
	v.round++
	if v.round == 0 {
		clear(v.seen)
		v.round++
	}
	for _, member := range members.List() {
		if member == nil || member.ActorRef == nil {
			return false
		}
		index, exists := v.indexes[member.ID()]
		if !exists || v.seen[index] == v.round {
			return false
		}
		known := v.members.List()[index]
		if known.DataCenter != member.DataCenter || known.Status != member.Status || known.CreatedAt.UnixNano() != member.CreatedAt.UnixNano() ||
			known.ActorRef.GetAddress() != member.ActorRef.GetAddress() || known.ActorRef.GetPath() != member.ActorRef.GetPath() || known.ActorRef.IsVirtual() != member.ActorRef.IsVirtual() {
			return false
		}
		v.seen[index] = v.round
	}
	return true
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
