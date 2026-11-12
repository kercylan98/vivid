package gossipmessages

func NewSpreadGossip(gossipType string) *SpreadGossip {
	return &SpreadGossip{
		GossipType: gossipType,
	}
}

// SpreadGossip 表示立即开始 gossip 消息的扩散。
type SpreadGossip struct {
	GossipType string
}
