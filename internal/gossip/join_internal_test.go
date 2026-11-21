package gossip

import (
	"net"
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

type joinScheduler struct {
	vivid.Scheduler
	once func(vivid.ActorRef, time.Duration, vivid.Message, ...vivid.ScheduleOption)
}

func (*joinScheduler) Loop(vivid.ActorRef, time.Duration, vivid.Message, ...vivid.ScheduleOption) (vivid.ScheduledTask, error) {
	return nil, nil
}

func (*joinScheduler) Exists(string) bool { return false }

func (s *joinScheduler) Once(receiver vivid.ActorRef, delay time.Duration, message vivid.Message, options ...vivid.ScheduleOption) (vivid.ScheduledTask, error) {
	if s.once != nil {
		s.once(receiver, delay, message, options...)
	}
	return nil, nil
}

type joinContext struct{ tombstoneContext }

func (*joinContext) Scheduler() vivid.Scheduler { return &joinScheduler{} }

func TestActor_GossipJoinResponse(t *testing.T) {
	for _, name := range []string{"accepted", "expired", "ordinary_pong", "light_ack", "wrong_seed", "seed_leaving", "seed_regression", "missing_ack", "old_instance_ack", "wrong_revision", "nil_info", "nil_members", "nil_version", "wrong_cluster", "missing_cluster", "nil_pong", "rejected", "same_cluster_rejection", "nil_rejection", "invalid_data_center", "wrong_data_center_ack", "invalid_seed_status", "invalid_self_ref", "invalid_self_created", "invalid_relay", "nil_relay"} {
		t.Run(name, func(t *testing.T) {
			local := endpoint.NewInformation(&tombstoneRef{address: "local:10001"}, time.Unix(1, 0))
			local.Status = endpoint.StatusJoining
			local.DataCenter = "dc-a"
			seed := endpoint.NewInformation(&tombstoneRef{address: "seed:10002"}, time.Unix(2, 0))
			seed.Status = endpoint.StatusUp
			seed.DataCenter = "dc-b"
			a := New(vivid.NewActorSystemClusterOptions(vivid.WithActorSystemClusterName("vivid-test")))
			a.info, a.view, a.seeds = local, NewClusterView(log.GetDefault()), []vivid.ActorRef{seed.ActorRef}
			a.view.Members().Upsert(local)
			a.view.Version().Update(local.ID(), 1)
			a.joinRequest = future.NewFuture[vivid.Message](nil, 0, nil)
			a.joinAttempt = 1
			a.joinDeadline = time.Now().Add(time.Hour)
			if name == "accepted" {
				a.joinErrors = []error{vivid.ErrorFutureTimeout}
			}
			ctx := &joinContext{tombstoneContext{ref: local.ActorRef}}
			members := memberlist.New(log.GetDefault())
			members.Upsert(local.Clone())
			members.Upsert(seed)
			version := versionvector.New()
			version.Update(local.ID(), 1)
			version.Update(seed.ID(), 2)
			pong := gossipmessages.NewPong("vivid-test", gossipmessages.GossipTypeJoin, seed, members, version)
			switch name {
			case "expired":
				a.joinDeadline = time.Now().Add(-time.Second)
			case "ordinary_pong":
				pong.GossipType = gossipmessages.GossipTypeCycleSYNC
			case "wrong_seed":
				pong.Info = seed.Clone()
				pong.Info.ActorRef = &tombstoneRef{address: "other:10003"}
			case "seed_leaving":
				pong.Info.Status = endpoint.StatusLeaving
			case "seed_regression":
				known := seed.Clone()
				known.Status = endpoint.StatusRemoved
				a.view.Members().Upsert(known)
				a.view.Version().Update(known.ID(), 1)
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
			case "invalid_data_center":
				pong.Info.DataCenter = "dc b"
			case "invalid_seed_status":
				pong.Info.Status = endpoint.Status(255)
			case "invalid_self_ref":
				self := pong.MemberList.Get(local.ID())
				self.ActorRef = &viewPathRef{tombstoneRef: &tombstoneRef{address: local.ActorRef.GetAddress()}}
				pong.MemberList.Upsert(self)
			case "invalid_self_created":
				self := pong.MemberList.Get(local.ID())
				self.CreatedAt = local.CreatedAt.Add(time.Second)
				pong.MemberList.Upsert(self)
			case "invalid_relay", "nil_relay":
				remote := reachableMember("third:10003")
				remote.Status = endpoint.Status(255)
				pong.MemberList.Upsert(remote)
				pong.Version.Update(remote.ID(), 1)
				if name == "nil_relay" {
					records := pong.MemberList.List()
					records[2], records[0] = records[0], nil
					pong.MemberList = memberRecords(records)
				}
			case "wrong_data_center_ack":
				self := pong.MemberList.Get(local.ID())
				self.DataCenter = "dc-b"
				pong.MemberList.Upsert(self)
			}
			var message vivid.Message = pong
			switch name {
			case "light_ack":
				message = &gossipmessages.PongAck{ClusterName: "vivid-test", Info: seed, Revision: 2}
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
			a.onJoinReply(ctx, joinReply{instanceID: local.ID(), attempt: 2, message: message})
			require.NotNil(t, a.joinRequest)
			require.Equal(t, endpoint.StatusJoining, local.Status)
			a.onJoinReply(ctx, joinReply{instanceID: local.ID(), attempt: 1, message: message})
			result, err := a.Readiness().Wait(nil, time.Second).Result()
			if name == "accepted" || name == "invalid_relay" || name == "nil_relay" {
				assert.Len(t, a.view.Members().List(), 2, "加入时同样忽略无效转发记录")
				require.NoError(t, err)
				assert.Nil(t, a.joinErrors, "后续种子成功应释放此前失败原因")
				assert.Equal(t, seed.ActorRef.GetAddress(), result.SeedAddress)
				assert.False(t, result.Bootstrapped)
				assert.Equal(t, endpoint.StatusUp, local.Status)
			} else {
				assert.ErrorIs(t, err, vivid.ErrorClusterJoinFailed)
				if name == "expired" {
					assert.ErrorIs(t, err, vivid.ErrorFutureTimeout)
					assert.Zero(t, a.view.Version().Get(seed.ID()), "过期响应不得导入种子版本")
					assert.Nil(t, ctx.stream, "过期响应不得发布成员变更")
				}
				if name == "wrong_cluster" || name == "missing_cluster" || name == "rejected" {
					assert.ErrorIs(t, err, vivid.ErrorClusterNameMismatch)
				}
				assert.Equal(t, endpoint.StatusJoining, local.Status)
				expected := 1
				if name == "seed_regression" {
					expected = 2
					assert.Equal(t, endpoint.StatusRemoved, a.view.Members().Get(seed.ID()).Status)
					assert.Equal(t, uint64(1), a.view.Version().Get(seed.ID()))
				}
				assert.Equal(t, expected, a.view.Members().Len(), "无效加入响应不得污染成员视图")
			}
		})
	}
}

type joinFailureContext struct {
	joinContext
	requests int
	retry    vivid.Message
	delay    time.Duration
}

func (c *joinFailureContext) Scheduler() vivid.Scheduler {
	return &joinScheduler{once: func(_ vivid.ActorRef, delay time.Duration, message vivid.Message, _ ...vivid.ScheduleOption) {
		c.retry, c.delay = message, delay
	}}
}

func (c *joinFailureContext) Ask(vivid.ActorRef, vivid.Message, ...time.Duration) vivid.Future[vivid.Message] {
	c.requests++
	request := future.NewFuture[vivid.Message](nil, 0, nil)
	request.Close(vivid.ErrorFutureTimeout)
	return request
}

func TestActor_GossipJoinFailureCauses(t *testing.T) {
	for _, stop := range []bool{false, true} {
		name := "all_failed"
		if stop {
			name = "stopped"
		}
		t.Run(name, func(t *testing.T) {
			local := endpoint.NewInformation(&tombstoneRef{address: "local:10001"}, time.Unix(1, 0))
			local.Status = endpoint.StatusJoining
			a := New(vivid.NewActorSystemClusterOptions(vivid.WithActorSystemClusterName("vivid-test")))
			a.info, a.view = local, NewClusterView(log.GetDefault())
			a.view.Members().Upsert(local)
			a.seeds = []vivid.ActorRef{&tombstoneRef{address: "first:10002"}, &tombstoneRef{address: "second:10003"}}
			a.phaseKillCompleted = make(chan struct{})
			ctx := &joinFailureContext{joinContext: joinContext{tombstoneContext{ref: local.ActorRef}}}
			a.tryNextSeed(ctx)
			cause := &net.OpError{Op: "dial", Net: "tcp", Err: vivid.ErrorIllegalArgument}
			if stop {
				a.onJoinReply(ctx, joinReply{instanceID: local.ID(), index: 0, attempt: 1, err: cause})
				require.NotNil(t, ctx.retry)
				require.Positive(t, ctx.delay)
				a.onKill(ctx, &vivid.OnKill{})
				a.onJoinRetry(ctx, ctx.retry.(joinRetry))
				_, err := a.Readiness().Wait(nil, time.Second).Result()
				assert.ErrorIs(t, err, vivid.ErrorActorSystemStopped)
				assert.NotErrorIs(t, err, vivid.ErrorClusterJoinFailed)
				assert.Nil(t, a.joinErrors)
				assert.Equal(t, 1, ctx.requests, "停止后不得执行已排队的重试")
				return
			}
			a.onJoinReply(ctx, joinReply{instanceID: local.ID(), index: 0, attempt: 1, err: cause})
			require.Equal(t, 1, ctx.requests, "传输错误应等待退避后再重试")
			retry := ctx.retry
			require.NotNil(t, retry)
			ctx.retry = nil
			a.onJoinRetry(ctx, retry.(joinRetry))
			require.Equal(t, 2, ctx.requests)
			a.onJoinReply(ctx, joinReply{instanceID: local.ID(), index: 0, attempt: 2, err: cause})
			retry = ctx.retry
			require.NotNil(t, retry)
			ctx.retry = nil
			a.onJoinRetry(ctx, retry.(joinRetry))
			require.Equal(t, 3, ctx.requests)
			a.onJoinReply(ctx, joinReply{instanceID: local.ID(), index: 0, attempt: 3, err: cause})
			require.Equal(t, 4, ctx.requests, "同一个种子失败前应使用完三次请求预算，再尝试下一个种子")
			// 旧种子、旧请求和旧实例的结果不能结束新种子的当前请求。
			for _, reply := range []joinReply{
				{instanceID: local.ID(), index: 0, attempt: 3, err: cause},
				{instanceID: local.ID(), index: 1, attempt: 0, err: cause},
				{instanceID: "old-local", index: 1, attempt: 1, err: cause},
			} {
				a.onJoinReply(ctx, reply)
			}
			require.Len(t, a.joinErrors, 1)
			for attempt := 1; attempt <= joinAttemptsPerSeed; attempt++ {
				a.onJoinReply(ctx, joinReply{instanceID: local.ID(), index: 1, attempt: attempt, err: vivid.ErrorFutureTimeout})
			}
			_, err := a.Readiness().Wait(nil, time.Second).Result()
			require.ErrorIs(t, err, vivid.ErrorClusterJoinFailed)
			assert.ErrorIs(t, err, vivid.ErrorFutureTimeout)
			assert.ErrorIs(t, err, vivid.ErrorIllegalArgument)
			var operation *net.OpError
			require.ErrorAs(t, err, &operation)
			assert.Same(t, cause, operation, "聚合后仍保留原始类型错误")
			assert.Equal(t, 2*joinAttemptsPerSeed, ctx.requests)
			assert.Nil(t, a.joinErrors, "最终原因由只读就绪结果持有，不额外保留收集切片")
		})
	}
}

func TestActor_GossipJoinExpiredReplyNextSeed(t *testing.T) {
	local := reachableMember("local:10001")
	local.Status = endpoint.StatusJoining
	first, second := reachableMember("first:10002"), reachableMember("second:10003")
	a := New(vivid.NewActorSystemClusterOptions(vivid.WithActorSystemClusterName("vivid-test")))
	a.info, a.view = local, NewClusterView(log.GetDefault())
	a.view.Members().Upsert(local)
	a.view.Version().Update(local.ID(), 1)
	a.seeds = []vivid.ActorRef{first.ActorRef, second.ActorRef}
	a.joinAttempt, a.joinDeadline = 1, time.Now().Add(-time.Second)
	a.joinRequest = future.NewFuture[vivid.Message](nil, 0, nil)
	ctx := &joinFailureContext{joinContext: joinContext{tombstoneContext{ref: local.ActorRef}}}
	response := func(seed *endpoint.Information, index int) joinReply {
		members := memberlist.New(log.GetDefault())
		members.Upsert(local)
		members.Upsert(seed)
		version := versionvector.New()
		version.Update(local.ID(), 1)
		version.Update(seed.ID(), 2)
		return joinReply{instanceID: local.ID(), index: index, attempt: 1,
			message: gossipmessages.NewPong("vivid-test", gossipmessages.GossipTypeJoin, seed, members, version)}
	}
	late := response(first, 0)
	a.onJoinReply(ctx, late)
	require.Equal(t, 1, a.joinIndex)
	require.Equal(t, 1, ctx.requests)
	require.Len(t, a.joinErrors, 1)
	require.ErrorIs(t, a.joinErrors[0], vivid.ErrorFutureTimeout)
	require.Equal(t, endpoint.StatusJoining, local.Status)
	require.Equal(t, 1, a.view.Members().Len())
	require.Nil(t, ctx.stream)
	request := a.joinRequest
	// 上一种子的响应与退避通知重放不得影响下一种子的新预算和请求。
	deadline := a.joinDeadline
	a.onJoinReply(ctx, late)
	a.onJoinRetry(ctx, joinRetry{instanceID: local.ID(), index: 0, attempt: 1})
	require.Same(t, request, a.joinRequest)
	require.Equal(t, deadline, a.joinDeadline)
	require.Equal(t, 1, ctx.requests)
	a.onJoinReply(ctx, response(second, 1))
	result, err := a.Readiness().Wait(nil, time.Second).Result()
	require.NoError(t, err)
	require.Equal(t, second.ActorRef.GetAddress(), result.SeedAddress)
	require.Equal(t, endpoint.StatusUp, local.Status)
	require.Zero(t, a.view.Version().Get(first.ID()))
	require.Equal(t, 2, a.view.Members().Len())
	require.Nil(t, a.joinErrors)
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
