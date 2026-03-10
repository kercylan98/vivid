package endpoint

type Status uint8

const (
	StatusUnknown Status = iota // 未知状态
	StatusJoining               // 加入中
	StatusUp                    // 正常
)

// statusNexts 状态转移表，记录每个状态可以转移到哪些状态，并且是否需要立即广播一轮 gossip
var statusNexts = map[Status]map[Status]bool{
	StatusUnknown: {
		StatusJoining: false, // 新创建的节点可以加入集群
		StatusUp:      true,  // 新创建的节点自成集群
	},
	StatusJoining: {
		StatusUp: true, // 加入中可以升级为正常
	},
}

func (s Status) String() string {
	switch s {
	case StatusUnknown:
		return "UNKNOWN"
	case StatusJoining:
		return "JOINING"
	case StatusUp:
		return "UP"
	default:
		return "UNKNOWN"
	}
}

func (s Status) CanTransitionTo(next Status) (can bool, needGossip bool) {
	needGossip, can = statusNexts[s][next]
	return
}
