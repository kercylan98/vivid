package endpoint

// Status 节点在集群中的生命周期状态，用于 gossip 状态机与视图中的成员状态。
type Status uint8

const (
	StatusNone    Status = iota // 初始态，尚未进入 Joining 或 Up
	StatusJoining               // 加入中，正在向种子发 Ping 拉取视图
	StatusUp                    // 已上线，参与周期 gossip
	StatusLeaving               // 即将离开集群，正在处理离开集群的逻辑
	StatusExiting               // 已离开集群，正在处理离开集群的逻辑
	StatusRemoved               // 已从集群中移除，已完全离开集群
)

// String 返回状态的可读名称，用于日志与调试。
func (s Status) String() string {
	switch s {
	case StatusNone:
		return "NONE"
	case StatusJoining:
		return "JOINING"
	case StatusUp:
		return "UP"
	case StatusLeaving:
		return "LEAVING"
	case StatusExiting:
		return "EXITING"
	case StatusRemoved:
		return "REMOVED"
	default:
		return "NONE"
	}
}
