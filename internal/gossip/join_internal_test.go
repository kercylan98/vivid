package gossip

import (
	"slices"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/future"
	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/internal/gossip/gossipmessages"
	"github.com/kercylan98/vivid/internal/gossip/memberlist"
	"github.com/kercylan98/vivid/internal/gossip/versionvector"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type joinScheduler struct{ vivid.Scheduler }

func (*joinScheduler) Loop(vivid.ActorRef, time.Duration, vivid.Message, ...vivid.ScheduleOption) error {
	return nil
}

type joinContext struct{ tombstoneContext }

func (*joinContext) Scheduler() vivid.Scheduler { return &joinScheduler{} }

func TestActor_GossipJoinResponse(t *testing.T) {
	for _, name := range []string{"accepted", "ordinary_pong", "wrong_seed", "seed_leaving", "missing_ack", "old_instance_ack", "wrong_revision", "nil_info", "nil_members", "nil_version", "wrong_cluster", "missing_cluster", "nil_pong", "rejected", "same_cluster_rejection", "nil_rejection"} {
		t.Run(name, func(t *testing.T) {
			local := endpoint.NewInformation(&tombstoneRef{address: "local:10001"}, time.Unix(1, 0))
			local.Status = endpoint.StatusJoining
			seed := endpoint.NewInformation(&tombstoneRef{address: "seed:10002"}, time.Unix(2, 0))
			seed.Status = endpoint.StatusUp
			a := New(vivid.NewActorSystemClusterOptions(vivid.WithActorSystemClusterName("vivid-test")))
			a.info, a.view, a.seeds = local, NewClusterView(log.GetDefault()), []vivid.ActorRef{seed.ActorRef}
			a.view.Members().Upsert(local)
			a.view.Version().Update(local.ID(), 1)
			a.joinRequest = future.NewFuture[vivid.Message](nil, 0, nil)
			ctx := &joinContext{tombstoneContext{ref: local.ActorRef}}
			members := memberlist.New(log.GetDefault())
			members.Upsert(local.Clone())
			members.Upsert(seed)
			version := versionvector.New()
			version.Update(local.ID(), 1)
			version.Update(seed.ID(), 2)
			pong := gossipmessages.NewPong("vivid-test", gossipmessages.GossipTypeJoin, seed, members, version)
			switch name {
			case "ordinary_pong":
				pong.GossipType = gossipmessages.GossipTypeCycleSYNC
			case "wrong_seed":
				pong.Info = seed.Clone()
				pong.Info.ActorRef = &tombstoneRef{address: "other:10003"}
			case "seed_leaving":
				pong.Info.Status = endpoint.StatusLeaving
			case "missing_ack":
				pong.MemberList = memberlist.New(log.GetDefault())
			case "old_instance_ack":
				old := endpoint.NewInformation(local.ActorRef.Clone(), local.CreatedAt)
				old.Status = endpoint.StatusJoining
				pong.MemberList = memberlist.New(log.GetDefault())
				pong.MemberList.Upsert(old)
			case "wrong_revision":
				pong.Version.Update(local.ID(), 5)
			case "nil_info":
				pong.Info = nil
			case "nil_members":
				pong.MemberList = nil
			case "nil_version":
				pong.Version = nil
			case "wrong_cluster":
				pong.ClusterName = "Vivid-test"
			case "missing_cluster":
				pong.ClusterName = ""
			case "nil_pong":
				pong = nil
			}
			var message vivid.Message = pong
			switch name {
			case "rejected":
				message = &gossipmessages.JoinRejected{ClusterName: "other"}
			case "same_cluster_rejection":
				message = &gossipmessages.JoinRejected{ClusterName: a.opts.Name}
			case "nil_rejection":
				message = (*gossipmessages.JoinRejected)(nil)
			}
			// 与当前请求不匹配的返回不能结束等待或改变本实例状态。
			a.onJoinReply(ctx, joinReply{instanceID: "old-local-instance", message: message})
			a.onJoinReply(ctx, joinReply{instanceID: local.ID(), index: 1, message: message})
			a.onJoinReply(ctx, joinReply{instanceID: local.ID(), attempt: 1, message: message})
			require.NotNil(t, a.joinRequest)
			require.Equal(t, endpoint.StatusJoining, local.Status)
			a.onJoinReply(ctx, joinReply{instanceID: local.ID(), message: message})
			result, err := a.Readiness().Wait(nil, time.Second).Result()
			if name == "accepted" {
				require.NoError(t, err)
				assert.Equal(t, seed.ActorRef.GetAddress(), result.SeedAddress)
				assert.False(t, result.Bootstrapped)
				assert.Equal(t, endpoint.StatusUp, local.Status)
			} else {
				assert.ErrorIs(t, err, vivid.ErrorClusterJoinFailed)
				if name == "wrong_cluster" || name == "missing_cluster" || name == "rejected" {
					assert.ErrorIs(t, err, vivid.ErrorClusterNameMismatch)
				}
				assert.Equal(t, endpoint.StatusJoining, local.Status)
				assert.Equal(t, 1, len(a.view.Members().List()), "无效加入响应不得污染成员视图")
			}
		})
	}
}

type seedSystem struct{ vivid.ActorSystem }

func (*seedSystem) CreateRef(address, _ string) (vivid.ActorRef, error) {
	return &tombstoneRef{address: address}, nil
}

type seedContext struct {
	vivid.PrelaunchContext
	ref vivid.ActorRef
}

func (c *seedContext) Ref() vivid.ActorRef     { return c.ref }
func (*seedContext) System() vivid.ActorSystem { return &seedSystem{} }

func TestActor_GossipSeedNormalization(t *testing.T) {
	options := vivid.NewActorSystemClusterOptions(vivid.WithActorSystemClusterName("vivid-test"), vivid.WithActorSystemClusterSeeds(" z.example:10003 ", "local.example:10001", "a.example:10002", "a.example:10002"))
	original := slices.Clone(options.Seeds)
	a := New(options)
	ctx := &seedContext{ref: &tombstoneRef{address: "local.example:10001"}}
	require.NoError(t, a.initializeSeeds(ctx))
	require.Len(t, a.seeds, 2)
	assert.Equal(t, "a.example:10002", a.seeds[0].GetAddress())
	assert.Equal(t, "z.example:10003", a.seeds[1].GetAddress())
	assert.Equal(t, original, options.Seeds, "内部排序和去重不得修改调用方配置")
	options.Seeds[0] = "modified.example:10004"
	options.JoinTimeout = time.Hour
	assert.NotEqual(t, options.Seeds[0], a.opts.Seeds[0])
	assert.NotEqual(t, options.JoinTimeout, a.opts.JoinTimeout)

	selfOnly := New(vivid.NewActorSystemClusterOptions(vivid.WithActorSystemClusterName("vivid-test"), vivid.WithActorSystemClusterSeeds("local.example:10001")))
	assert.ErrorIs(t, selfOnly.initializeSeeds(ctx), vivid.ErrorIllegalArgument)
	selfOnly.opts.Bootstrap = true
	assert.NoError(t, selfOnly.initializeSeeds(ctx))
}
