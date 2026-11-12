package endpoint

import "github.com/kercylan98/vivid"

// Status 与公开成员快照共用生命周期类型。
type Status = vivid.ClusterMemberStatus

const (
	StatusNone    = vivid.ClusterMemberStatusNone
	StatusJoining = vivid.ClusterMemberStatusJoining
	StatusUp      = vivid.ClusterMemberStatusUp
	StatusLeaving = vivid.ClusterMemberStatusLeaving
	StatusExiting = vivid.ClusterMemberStatusExiting
	StatusRemoved = vivid.ClusterMemberStatusRemoved
)
