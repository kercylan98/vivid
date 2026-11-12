package gossip

import (
	"testing"
	"time"

	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/internal/gossip/gossipmessages"
	"github.com/kercylan98/vivid/internal/gossip/memberlist"
	"github.com/kercylan98/vivid/internal/gossip/versionvector"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActor_GossipInstanceIsolation(t *testing.T) {
	for _, oldStatus := range []endpoint.Status{endpoint.StatusUp, endpoint.StatusRemoved} {
		for _, reply := range []bool{false, true} {
			for _, relayed := range []bool{false, true} {
				name := oldStatus.String() + "/ping"
				if reply {
					name = oldStatus.String() + "/pong"
				}
				if relayed {
					name += "/relayed"
				}
				t.Run(name, func(t *testing.T) {
					local := endpoint.NewInformation(&tombstoneRef{address: "local:10001"}, time.Unix(1, 0))
					local.Status = endpoint.StatusUp
					old := endpoint.NewInformation(&tombstoneRef{address: "restarted:10002"}, time.Unix(3, 0))
					old.Status = oldStatus
					// 新实例的时钟可以回退，身份隔离不能依赖创建时间的大小。
					current := endpoint.NewInformation(old.ActorRef.Clone(), time.Unix(2, 0))
					current.Status = endpoint.StatusUp
					require.NotEqual(t, old.ID(), current.ID(), "同地址重启必须创建独立的版本命名空间")
					a := &Actor{info: local, view: NewClusterView(log.GetDefault())}
					a.view.Members().Upsert(local)
					a.view.Members().Upsert(old)
					a.view.Version().Increment(local.ID())
					for i := 0; i < 5; i++ {
						a.view.Version().Increment(old.ID())
					}
					ctx := &tombstoneContext{ref: local.ActorRef}
					members := memberlist.New(log.GetDefault())
					members.Upsert(current)
					version := versionvector.New()
					version.Increment(current.ID())
					version.Increment(current.ID())
					sender := current
					if relayed {
						sender = endpoint.NewInformation(&tombstoneRef{address: "relay:10003"}, time.Unix(4, 0))
						sender.Status = endpoint.StatusUp
						members.Upsert(sender)
						version.Increment(sender.ID())
					}
					deliver := func() {
						if reply {
							a.onPong(ctx, gossipmessages.NewPong(gossipmessages.GossipTypeCycleSYNC, sender, members, version))
						} else {
							a.onPing(ctx, gossipmessages.NewPing(gossipmessages.GossipTypeCycleSYNC, sender, members, version))
						}
					}
					deliver()
					require.NotNil(t, a.view.Members().Get(current.ID()))
					assert.Equal(t, endpoint.StatusUp, a.view.Members().Get(current.ID()).Status)
					assert.Equal(t, uint64(2), a.view.Version().Get(current.ID()))
					// 迟到的旧实例消息只能更新旧实例，不能覆盖新实例或清除退出墓碑。
					sender = old.Clone()
					sender.Status = endpoint.StatusUp
					members = memberlist.New(log.GetDefault())
					members.Upsert(sender)
					version = versionvector.New()
					for i := 0; i < 3; i++ {
						version.Increment(sender.ID())
					}
					deliver()
					assert.Equal(t, oldStatus, a.view.Members().Get(old.ID()).Status)
					assert.Equal(t, uint64(5), a.view.Version().Get(old.ID()))
					assert.Equal(t, endpoint.StatusUp, a.view.Members().Get(current.ID()).Status)
					assert.Equal(t, uint64(2), a.view.Version().Get(current.ID()))
				})
			}
		}
	}
}

func TestActor_GossipLocalVersionOwnership(t *testing.T) {
	for _, reply := range []bool{false, true} {
		for _, direct := range []bool{false, true} {
			name := "ping"
			if reply {
				name = "pong"
			}
			if direct {
				name += "/self_sender"
			}
			t.Run(name, func(t *testing.T) {
				local := endpoint.NewInformation(&tombstoneRef{address: "local:10001"}, time.Unix(1, 0))
				local.Status = endpoint.StatusUp
				a := &Actor{info: local, view: NewClusterView(log.GetDefault())}
				a.view.Members().Upsert(local)
				a.view.Version().Increment(local.ID())
				ctx := &tombstoneContext{ref: local.ActorRef}
				remoteSelf := local.Clone()
				remoteSelf.Status = endpoint.StatusRemoved
				members := memberlist.New(log.GetDefault())
				members.Upsert(remoteSelf)
				version := versionvector.New()
				for i := 0; i < 5; i++ {
					version.Increment(local.ID())
				}
				version.Increment("unknown-without-member")
				sender := remoteSelf
				if !direct {
					sender = endpoint.NewInformation(&tombstoneRef{address: "relay:10003"}, time.Unix(3, 0))
					sender.Status = endpoint.StatusUp
					members.Upsert(sender)
					version.Increment(sender.ID())
				}
				if reply {
					a.onPong(ctx, gossipmessages.NewPong(gossipmessages.GossipTypeCycleSYNC, sender, members, version))
				} else {
					a.onPing(ctx, gossipmessages.NewPing(gossipmessages.GossipTypeCycleSYNC, sender, members, version))
				}
				assert.Same(t, local, a.view.Members().Get(local.ID()))
				assert.Equal(t, endpoint.StatusUp, local.Status)
				assert.Equal(t, uint64(1), a.view.Version().Get(local.ID()), "本实例的版本只能由本实例递增")
				assert.Zero(t, a.view.Version().Get("unknown-without-member"), "版本必须与成员记录一起接收")
			})
		}
	}
}
