package gossipmessages

var _JoinCluster = JoinCluster{}

func NewJoinCluster() *JoinCluster {
	return &_JoinCluster
}

// JoinCluster 表示立即开始加入集群。
type JoinCluster struct{}
