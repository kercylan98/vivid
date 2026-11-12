package memberlist_test

import (
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/actor"
	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/internal/gossip/memberlist"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemberList_UnseensUnknownSeeds(t *testing.T) {
	localRef, err := actor.NewRef("127.0.0.1:10001", "/@gossip")
	require.NoError(t, err)
	seed, err := actor.NewRef("127.0.0.1:10002", "/@gossip")
	require.NoError(t, err)
	local := endpoint.NewInformation(localRef, time.Now())
	local.Status = endpoint.StatusUp
	members := memberlist.New(log.GetDefault())
	members.Upsert(local)
	seeds := []vivid.ActorRef{nil, localRef, seed, seed.Clone()}
	assert.Equal(t, []vivid.ActorRef{seed}, members.Unseens(local, seeds, 5))
	assert.Nil(t, members.Get(seed.String()), "联络种子不应将其直接加入成员视图")
	assert.Empty(t, members.Unseens(local, seeds, 0))
	assert.Empty(t, members.Unseens(local, seeds, -1))
	// 离开期间不再探测未知种子，避免离线种子拖延正常退出。
	local.Status = endpoint.StatusLeaving
	assert.Empty(t, members.Unseens(local, seeds, 5))
	local.Status = endpoint.StatusUp
	removed := endpoint.NewInformation(seed, time.Now())
	removed.Status = endpoint.StatusJoining
	members.Upsert(removed)
	assert.Equal(t, []vivid.ActorRef{seed}, members.Unseens(local, seeds, 5), "过期的 Joining 视图不能阻止种子重试")
	removed.Status = endpoint.StatusRemoved
	members.Upsert(removed)
	assert.Empty(t, members.Unseens(local, seeds, 5), "保留已移除种子的墓碑语义")
}

func TestMemberList_UnseensCandidates(t *testing.T) {
	refs := make([]vivid.ActorRef, 5)
	for i, address := range []string{"127.0.0.1:10001", "127.0.0.1:10002", "127.0.0.1:10003", "127.0.0.1:10004", "127.0.0.1:10005"} {
		ref, err := actor.NewRef(address, "/@gossip")
		require.NoError(t, err)
		refs[i] = ref
	}
	members := memberlist.New(log.GetDefault())
	local := endpoint.NewInformation(refs[0], time.Now())
	local.Status = endpoint.StatusUp
	members.Upsert(local)
	for i, status := range []endpoint.Status{endpoint.StatusUp, endpoint.StatusLeaving, endpoint.StatusRemoved} {
		info := endpoint.NewInformation(refs[i+1], time.Now())
		info.Status = status
		members.Upsert(info)
	}
	seeds := []vivid.ActorRef{refs[0], refs[1], refs[3], refs[4], refs[4].Clone()}
	assert.ElementsMatch(t, []vivid.ActorRef{refs[1], refs[2], refs[4]}, members.Unseens(local, seeds, 10))
	assert.Len(t, members.Unseens(local, seeds, 1), 1)
}
