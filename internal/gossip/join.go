package gossip

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/internal/gossip/gossipmessages"
	"github.com/kercylan98/vivid/pkg/log"
)

const launchSchedulerReference = "launch"
const joinAttemptsPerSeed = 3

type launchJoin struct{ instanceID string }

type joinReply struct {
	instanceID string
	index      int
	attempt    int
	message    vivid.Message
	err        error
}

// Readiness 返回线程安全的初始加入结果，不暴露 Actor 的可变视图。
func (a *Actor) Readiness() *Readiness { return a.readiness }

func (a *Actor) initializeSeeds(ctx vivid.PrelaunchContext) error {
	if err := a.opts.Validate(); err != nil {
		return err
	}
	// 内部保留独立配置，排序和去重均不修改调用方选项。
	options := *a.opts
	options.Seeds = slices.Clone(a.opts.Seeds)
	a.opts = &options
	a.seeds = make([]vivid.ActorRef, 0, len(a.opts.Seeds))
	seen := make(map[string]struct{}, len(a.opts.Seeds))
	for _, seed := range a.opts.Seeds {
		ref, err := ctx.System().CreateRef(strings.TrimSpace(seed), "/"+gossipActorName)
		if err != nil {
			return err
		}
		if ref.Equals(ctx.Ref()) {
			continue
		}
		if _, exists := seen[ref.String()]; !exists {
			seen[ref.String()] = struct{}{}
			a.seeds = append(a.seeds, ref)
		}
	}
	slices.SortFunc(a.seeds, func(a, b vivid.ActorRef) int { return strings.Compare(a.GetAddress(), b.GetAddress()) })
	if len(a.opts.Seeds) > 0 && len(a.seeds) == 0 && !a.opts.Bootstrap {
		return vivid.ErrorIllegalArgument.WithMessage("cluster seeds contain only the local address; use bootstrap explicitly")
	}
	return nil
}

// onLaunch 只调度启动，不在 Actor 消息线程中睡眠。
func (a *Actor) onLaunch(ctx vivid.ActorContext) {
	if a.stopping || a.info.Status != endpoint.StatusNone {
		return
	}
	if a.opts.LaunchDelay == 0 {
		changeStatus(ctx, a, EventJoining)
		return
	}
	if err := ctx.Scheduler().Once(ctx.Ref(), a.opts.LaunchDelay, launchJoin{instanceID: a.info.ID()}, vivid.WithSchedulerReference(launchSchedulerReference)); err != nil {
		a.readiness.complete(vivid.ClusterJoinResult{}, vivid.ErrorGossipScheduleFailed.With(err))
	}
}

func (a *Actor) onJoining(ctx vivid.ActorContext) {
	if a.opts.Bootstrap || len(a.opts.Seeds) == 0 {
		changeStatus(ctx, a, EventUp)
		if a.joinError == nil {
			a.readiness.complete(vivid.ClusterJoinResult{LocalID: a.info.ID(), Bootstrapped: true}, nil)
		}
		return
	}
	a.tryNextSeed(ctx)
}

func (a *Actor) tryNextSeed(ctx vivid.ActorContext) {
	if a.stopping || a.info.Status != endpoint.StatusJoining {
		return
	}
	if a.joinIndex == len(a.seeds) {
		err := vivid.ErrorClusterJoinFailed.With(a.joinError)
		a.readiness.complete(vivid.ClusterJoinResult{}, err)
		ctx.Logger().Error("cluster join failed", log.Any("error", err))
		return
	}
	if a.joinAttempt == 0 {
		a.joinDeadline = time.Now().Add(a.opts.JoinTimeout)
	}
	remaining := time.Until(a.joinDeadline)
	if remaining <= 0 || a.joinAttempt == joinAttemptsPerSeed {
		a.joinError = vivid.ErrorFutureTimeout.WithMessage(a.opts.JoinTimeout.String())
		a.joinIndex++
		a.joinAttempt = 0
		a.tryNextSeed(ctx)
		return
	}
	// 一次请求可能随连接切换丢失；有限重发共享该种子的总超时，不延长加入预算。
	timeout := remaining / time.Duration(joinAttemptsPerSeed-a.joinAttempt)
	if timeout == 0 {
		timeout = remaining
	}
	a.joinAttempt++
	index, instanceID := a.joinIndex, a.info.ID()
	attempt := a.joinAttempt
	self := ctx.Ref()
	request := ctx.Ask(a.seeds[index], a.preparePing(gossipmessages.GossipTypeJoin), timeout)
	a.joinRequest = request
	// 后台仅等待 Future，将结果交回消息线程；不读写成员状态。
	go func() {
		message, err := request.Result()
		ctx.Tell(self, joinReply{instanceID: instanceID, index: index, attempt: attempt, message: message, err: err})
	}()
}

func (a *Actor) onJoinReply(ctx vivid.ActorContext, reply joinReply) {
	if a.stopping || a.info.Status != endpoint.StatusJoining || reply.instanceID != a.info.ID() || reply.index != a.joinIndex || reply.attempt != a.joinAttempt || a.joinRequest == nil {
		return
	}
	a.joinRequest = nil
	pong, ok := reply.message.(*gossipmessages.Pong)
	err := reply.err
	if err == nil && (!ok || !a.validJoinPong(pong)) {
		err = vivid.ErrorClusterJoinFailed.WithMessage(fmt.Sprintf("invalid join response from %s", a.seeds[a.joinIndex].GetAddress()))
	}
	if err != nil {
		a.joinError = err
		if errors.Is(err, vivid.ErrorFutureTimeout) {
			a.tryNextSeed(ctx)
			return
		}
		a.joinIndex++
		a.joinAttempt = 0
		a.tryNextSeed(ctx)
		return
	}
	seed := a.seeds[a.joinIndex].GetAddress()
	a.mergePong(ctx, pong)
	a.joinError = nil
	changeStatus(ctx, a, EventUp)
	if a.joinError == nil {
		a.readiness.complete(vivid.ClusterJoinResult{LocalID: a.info.ID(), SeedAddress: seed}, nil)
	}
}

func (a *Actor) validJoinPong(pong *gossipmessages.Pong) bool {
	if pong == nil || pong.Info == nil || pong.Info.ActorRef == nil || pong.MemberList == nil || pong.Version == nil ||
		pong.GossipType != gossipmessages.GossipTypeJoin || pong.Info.Status != endpoint.StatusUp || pong.Info.ID() == "" ||
		pong.Info.ID() == a.info.ID() || !pong.Info.ActorRef.Equals(a.seeds[a.joinIndex]) || pong.Version.Get(pong.Info.ID()) == 0 {
		return false
	}
	// 种子应确认收到本实例的 Joining 记录；普通或旧实例的 Pong 不构成加入成功。
	self := pong.MemberList.Get(a.info.ID())
	return self != nil && self.ActorRef != nil && self.ActorRef.Equals(a.info.ActorRef) && self.Status == endpoint.StatusJoining &&
		pong.Version.Get(a.info.ID()) == a.view.Version().Get(a.info.ID())
}
