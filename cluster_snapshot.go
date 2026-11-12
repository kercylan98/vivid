package vivid

import "time"

// ClusterMemberStatus 表示成员实例的生命周期，不代表实时可达性或写入资格。
type ClusterMemberStatus uint8

const (
	ClusterMemberStatusNone    ClusterMemberStatus = iota // 尚未开始加入
	ClusterMemberStatusJoining                            // 正在加入
	ClusterMemberStatusUp                                 // 已完成加入或自举
	ClusterMemberStatusLeaving                            // 正在离开
	ClusterMemberStatusExiting                            // 正在推进退出
	ClusterMemberStatusRemoved                            // 已完成本实例退出
)

// String 返回生命周期状态的名称。
func (s ClusterMemberStatus) String() string {
	switch s {
	case ClusterMemberStatusJoining:
		return "JOINING"
	case ClusterMemberStatusUp:
		return "UP"
	case ClusterMemberStatusLeaving:
		return "LEAVING"
	case ClusterMemberStatusExiting:
		return "EXITING"
	case ClusterMemberStatusRemoved:
		return "REMOVED"
	default:
		return "NONE"
	}
}

// ClusterMember 是本地观察到的成员实例副本，同一地址可以保留多个实例。
type ClusterMember struct {
	ID        string              // 本次启动的实例 ID
	Address   string              // Remoting 广告地址
	Status    ClusterMemberStatus // 生命周期状态
	Revision  uint64              // 该实例的状态版本
	CreatedAt time.Time           // 实例报告的创建时间，不用于授予接管资格
}

// ClusterSnapshot 是本地成员视图的只读快照，仅供本地查询和事件使用。
// Revision 只在相同 LocalID 下比较，不表示全局版本或多数派确认。
type ClusterSnapshot interface {
	LocalID() string                        // 当前观察节点的实例 ID
	Revision() uint64                       // 本地视图的变更序号
	Len() int                               // 包含历史实例及退出记录的成员数
	Member(id string) (ClusterMember, bool) // 按实例 ID 获取值副本
	Members() []ClusterMember               // 按实例 ID 排序的独立切片，可由调用方修改
}
