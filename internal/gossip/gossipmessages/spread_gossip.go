package gossipmessages

import "github.com/kercylan98/vivid"

var _SpreadGossip = SpreadGossip{}

func NewSpreadGossip() *SpreadGossip {
	return &_SpreadGossip
}

// SpreadGossip 表示立即开始 gossip 消息的扩散。
type SpreadGossip struct {
	Seeds []vivid.ActorRef
}
