package endpoint

// Status 节点在集群中的生命周期状态，用于 gossip 状态机与视图中的成员状态。
type Status uint8

const (
	StatusNone    Status = iota // 初始态，尚未进入 Joining 或 Up
	StatusJoining              // 加入中，正在向种子发 Ping 拉取视图
	StatusUp                   // 已上线，参与周期 gossip
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
	default:
		return "NONE"
	}
}
