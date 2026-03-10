package gossip

import (
	"fmt"
	"slices"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/gossip/internal/endpoint"
	"github.com/kercylan98/vivid/internal/gossip/internal/gossipmessages"
	"github.com/kercylan98/vivid/internal/gossip/internal/memberlist"
	"github.com/kercylan98/vivid/internal/gossip/internal/versionvector"
	"github.com/kercylan98/vivid/internal/utils"
	"github.com/kercylan98/vivid/pkg/log"
)

const (
	defaultGossipInterval = time.Second / 5
)

var (
	_ vivid.Actor          = (*Actor)(nil)
	_ vivid.PrelaunchActor = (*Actor)(nil)
)

func New(seeds ...vivid.ActorRef) *Actor {
	return &Actor{
		seeds: slices.DeleteFunc(seeds, func(seed vivid.ActorRef) bool {
			return seed == nil
		}),
		memberList: memberlist.New(),
		version:    versionvector.New(),
		backoff:    utils.NewExponentialBackoffWithDefault(time.Second, time.Minute),
	}
}

// Actor 是 gossip 的 Actor 实现
type Actor struct {
	seeds      []vivid.ActorRef             // 种子节点列表
	info       *endpoint.Information        // 自身节点信息
	memberList *memberlist.MemberList       // 自身维护的成员列表
	version    *versionvector.VersionVector // 该节点维护的版本向量
	backoff    *utils.ExponentialBackoff    // 退避重试器
}

// OnPrelaunch implements [vivid.PrelaunchActor].
func (a *Actor) OnPrelaunch(ctx vivid.PrelaunchContext) error {
	a.info = endpoint.NewInformation(ctx.Ref())
	return nil
}

func (a *Actor) setStatus(ctx vivid.ActorContext, status endpoint.Status) {
	changed, needGossip := a.info.SetStatus(status)
	if !changed {
		return
	}

	a.version.Increment(a.info.ID())
	a.onStatusChanged(ctx, status)

	if needGossip {
		a.ping(ctx)
	}
}

func (a *Actor) OnReceive(ctx vivid.ActorContext) {
	switch msg := ctx.Message().(type) {
	case *vivid.OnLaunch:
		a.onLaunch(ctx)
	case *gossipmessages.Ping:
		a.onPing(ctx, msg)
	case func(ctx vivid.ActorContext):
		msg(ctx)
	}
}

func (a *Actor) onStatusChanged(ctx vivid.ActorContext, status endpoint.Status) {
	switch status {
	case endpoint.StatusJoining:
		a.onJoining(ctx)
	case endpoint.StatusUp:
		a.onUp(ctx)
	}
}

func (a *Actor) onLaunch(ctx vivid.ActorContext) {
	// 如果存在种子节点，尝试加入种子节点，否则自成集群
	if len(a.seeds) > 0 {
		a.setStatus(ctx, endpoint.StatusJoining)
	} else {
		a.setStatus(ctx, endpoint.StatusUp)
	}
}

func (a *Actor) onJoining(ctx vivid.ActorContext) {
	// 尝试加入种子节点，拉取到节点信息后会自动更新至上线状态
	ctx.Logger().Debug("joining cluster", log.String("seeds", vivid.ActorRefs(a.seeds).String()))
	a.memberList.Add(a.info)
	a.ping(ctx, a.seeds...)

	// 假设未成功加入集群，则进行退避重试
	if a.info.Status == endpoint.StatusJoining {
		ctx.Logger().Debug("join cluster failed, will retry with backoff", log.String("backoff", a.backoff.Next().String()))
		ctx.Scheduler().Once(ctx.Ref(), a.backoff.Next(), func(ctx vivid.ActorContext) {
			a.onJoining(ctx)
		})
	}
}

func (a *Actor) onUp(ctx vivid.ActorContext) {
	ctx.Logger().Debug("joined cluster")
	// 上线后开始 gossip
	ctx.Scheduler().Loop(ctx.Ref(), defaultGossipInterval, func(ctx vivid.ActorContext) {
		a.ping(ctx)
	})
}

func (a *Actor) ping(ctx vivid.ActorContext, targets ...vivid.ActorRef) {
	if len(targets) == 0 {
		// 获取未见过的节点列表
		peers := a.memberList.Unseens(a.info, 5) // 最多发送 5 个节点
		if len(peers) == 0 {
			return
		}

		for _, peer := range peers {
			targets = append(targets, peer.ActorRef)
		}
	}

	ping := gossipmessages.NewPing(a.info, a.memberList, a.version)

	var futures []vivid.Future[vivid.Message]
	for _, peer := range targets {
		futures = append(futures, ctx.Ask(peer, ping))
	}

	for i, future := range futures {
		result, err := future.Result()
		if err != nil {
			ctx.Logger().Error("failed to gossip", log.String("ref", targets[i].String()), log.String("error", err.Error()))
			continue
		}

		pong, ok := result.(*gossipmessages.Pong)
		if !ok {
			ctx.Logger().Error("failed to gossip", log.String("ref", targets[i].String()), log.String("error", fmt.Sprintf("invalid pong message: %T", result)))
			continue
		}

		// 如果自身版本向量早于其他节点版本向量，则合并其他节点版本向量
		if a.version.IsBefore(pong.Version) {
			a.memberList.Merge(pong.MemberList)
			a.version.Merge(pong.Version)
		}

		// 如果该节点处于加入中，且收到了加入的请求回复，则可更新至上线状态
		a.setStatus(ctx, endpoint.StatusUp)
	}

}

func (a *Actor) onPing(ctx vivid.ActorContext, msg *gossipmessages.Ping) {
	// 如果自身版本向量早于其他节点版本向量，则合并其他节点版本向量
	if a.version.IsBefore(msg.Version) {
		a.memberList.Merge(msg.MemberList)
		a.version.Merge(msg.Version)
	}

	pong := gossipmessages.NewPong(a.info, a.memberList, a.version)
	ctx.Reply(pong)
}
