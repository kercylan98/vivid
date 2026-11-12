package gossip

import (
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/internal/gossip/gossipmessages"
	"github.com/kercylan98/vivid/internal/gossip/memberlist"
	"github.com/kercylan98/vivid/internal/gossip/versionvector"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type tombstoneRef struct {
	vivid.ActorRef
	address string
}

func (r *tombstoneRef) String() string { return r.address + ":/@gossip" }
func (r *tombstoneRef) Clone() vivid.ActorRef {
	return &tombstoneRef{address: r.address}
}

type tombstoneContext struct {
	vivid.ActorContext
	ref vivid.ActorRef
}

func (c *tombstoneContext) Ref() vivid.ActorRef              { return c.ref }
func (*tombstoneContext) Logger() log.Logger                 { return log.GetDefault() }
func (*tombstoneContext) Tell(vivid.ActorRef, vivid.Message) {}
func (*tombstoneContext) Reply(vivid.Message)                {}

func TestActor_GossipRemovedVersion(t *testing.T) {
	for _, reply := range []bool{false, true} {
		for _, relayed := range []bool{false, true} {
			name := "ping"
			if reply {
				name = "pong"
			}
			if relayed {
				name += "/relayed"
			} else {
				name += "/direct"
			}
			t.Run(name, func(t *testing.T) {
				local := endpoint.NewInformation(&tombstoneRef{address: "local:10001"}, time.Unix(1, 0))
				local.Status = endpoint.StatusUp
				removed := endpoint.NewInformation(&tombstoneRef{address: "removed:10002"}, time.Unix(2, 0))
				removed.Status = endpoint.StatusRemoved
				view := NewClusterView(log.GetDefault())
				view.Members().Upsert(local)
				view.Members().Upsert(removed)
				view.Version().Increment(local.ID())
				for i := 0; i < 5; i++ {
					view.Version().Increment(removed.ID())
				}
				a := &Actor{info: local, view: view}
				ctx := &tombstoneContext{ref: local.ActorRef}
				stale := removed.Clone()
				stale.Status = endpoint.StatusUp
				members := memberlist.New(log.GetDefault())
				members.Upsert(stale)
				version := versionvector.New()
				for i := 0; i < 3; i++ {
					version.Increment(stale.ID())
				}
				sender := stale
				if relayed {
					sender = endpoint.NewInformation(&tombstoneRef{address: "relay:10003"}, time.Unix(3, 0))
					sender.Status = endpoint.StatusUp
					members.Upsert(sender)
					version.Increment(sender.ID())
				}
				// 本地视图稳定后，迟到的直接或转发视图仍不得覆盖更新的退出记录。
				for i := 0; i < 3; i++ {
					a.onConverged(ctx)
					assert.Equal(t, uint64(5), view.Version().Get(removed.ID()))
					if reply {
						a.onPong(ctx, gossipmessages.NewPong(gossipmessages.GossipTypeCycleSYNC, sender, members, version))
					} else {
						a.onPing(ctx, gossipmessages.NewPing(gossipmessages.GossipTypeCycleSYNC, sender, members, version))
					}
					require.Equal(t, endpoint.StatusRemoved, view.Members().Get(removed.ID()).Status)
					require.Equal(t, uint64(5), view.Version().Get(removed.ID()))
				}
				// 后续传播必须同时保留 Removed 状态和对应版本。
				ping := a.preparePing(gossipmessages.GossipTypeCycleSYNC)
				assert.Equal(t, endpoint.StatusRemoved, ping.MemberList.Get(removed.ID()).Status)
				assert.Equal(t, uint64(5), ping.Version.Get(removed.ID()))
			})
		}
	}
}
