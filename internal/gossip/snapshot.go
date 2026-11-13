package gossip

import (
	"slices"
	"strings"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/ves"
)

// SnapshotRequest 仅由本地系统查询使用，不加入远程消息注册表。
type SnapshotRequest struct{}

type clusterSnapshot struct {
	localID  string
	revision uint64
	members  []vivid.ClusterMember
}

func (s *clusterSnapshot) LocalID() string  { return s.localID }
func (s *clusterSnapshot) Revision() uint64 { return s.revision }
func (s *clusterSnapshot) Len() int         { return len(s.members) }

func (s *clusterSnapshot) Members() []vivid.ClusterMember { return slices.Clone(s.members) }

func (s *clusterSnapshot) Member(id string) (vivid.ClusterMember, bool) {
	index, found := slices.BinarySearchFunc(s.members, id, func(member vivid.ClusterMember, id string) int {
		return strings.Compare(member.ID, id)
	})
	if !found {
		return vivid.ClusterMember{}, false
	}
	return s.members[index], true
}

// currentSnapshot 仅在 Actor 消息线程内构造，后续查询共享只读结果。
func (a *Actor) currentSnapshot() *clusterSnapshot {
	if a.snapshot != nil {
		return a.snapshot
	}
	members := make([]vivid.ClusterMember, len(a.view.Members().List()))
	for i, member := range a.view.Members().List() {
		members[i] = vivid.ClusterMember{
			ID:           member.ID(),
			Address:      member.ActorRef.GetAddress(),
			Status:       member.Status,
			Revision:     a.view.Version().Get(member.ID()),
			CreatedAt:    member.CreatedAt,
			Reachability: a.reachability.state(member.ID()),
		}
	}
	slices.SortFunc(members, func(a, b vivid.ClusterMember) int { return strings.Compare(a.ID, b.ID) })
	a.snapshot = &clusterSnapshot{localID: a.info.ID(), revision: a.viewRevision, members: members}
	return a.snapshot
}

func (a *Actor) publishMembersChanged(ctx vivid.ActorContext) {
	a.viewRevision++
	a.snapshot = nil
	a.membersChanged = false
	ctx.EventStream().Publish(ctx, ves.ClusterMembersChangedEvent{Snapshot: a.currentSnapshot()})
	a.publishReachabilityChanged(ctx)
}
