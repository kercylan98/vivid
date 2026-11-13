package gossip_test

import (
	"net"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/gossip"
	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/internal/gossip/gossipmessages"
	"github.com/kercylan98/vivid/pkg/bootstrap"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMemberValidationSystems(t *testing.T) ([]vivid.PrimaryActorSystem, []vivid.ActorRef, func(int) gossip.SnapshotResponse) {
	t.Helper()
	var systems []vivid.PrimaryActorSystem
	var refs []vivid.ActorRef
	for range 2 {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		address := listener.Addr().String()
		require.NoError(t, listener.Close())
		system := bootstrap.NewActorSystem(
			vivid.WithActorSystemRemoting(address),
			vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
		)
		require.NoError(t, system.Start())
		t.Cleanup(func() { require.NoError(t, system.Stop(3*time.Second)) })
		member := gossip.NewSnapshotActor(vivid.NewActorSystemClusterOptions(
			vivid.WithActorSystemClusterName("vivid-test"),
			vivid.WithActorSystemClusterLaunchDelay(0),
			vivid.WithActorSystemClusterGossipInterval(time.Hour),
			vivid.WithActorSystemClusterGracefulShutdownTimeout(50*time.Millisecond),
		))
		ref, err := system.ActorOf(member)
		require.NoError(t, err)
		_, err = member.Readiness().Wait(system, time.Second).Result()
		require.NoError(t, err)
		systems, refs = append(systems, system), append(refs, ref)
	}
	read := func(index int) gossip.SnapshotResponse {
		message, err := systems[index].Ask(refs[index], gossip.TestSnapshotRequest{}, time.Second).Result()
		require.NoError(t, err)
		return message.(gossip.SnapshotResponse)
	}
	return systems, refs, read
}

func TestActor_GossipMemberValidationTCP(t *testing.T) {
	systems, refs, read := newMemberValidationSystems(t)
	origin := read(0)
	remote := origin.View.Members().Get(origin.ID)
	for _, protocol := range []string{"ping", "pong"} {
		t.Run(protocol, func(t *testing.T) {
			bad := remote.Clone()
			bad.Status = endpoint.Status(255)
			version := origin.View.Version().Clone()
			version.Increment(bad.ID())
			// 无效成员先经实际注册 Codec 和 TCP，到达后用同源合法 Ping 确认处理顺序。
			if protocol == "ping" {
				systems[0].Tell(refs[1], gossipmessages.NewPing("vivid-test", gossipmessages.GossipTypeCycleSYNC, bad, origin.View.Members(), version))
			} else {
				systems[0].Tell(refs[1], gossipmessages.NewPong("vivid-test", gossipmessages.GossipTypeCycleSYNC, bad, origin.View.Members(), version))
			}
			message, err := systems[0].Ask(refs[1], gossipmessages.NewPing("vivid-test", gossipmessages.GossipTypeCycleSYNC, remote, origin.View.Members(), origin.View.Version()), time.Second).Result()
			require.NoError(t, err)
			pong, ok := message.(*gossipmessages.Pong)
			require.True(t, ok)
			assert.Equal(t, endpoint.StatusUp, pong.MemberList.Get(remote.ID()).Status)
			assert.Equal(t, origin.View.Version().Get(remote.ID()), pong.Version.Get(remote.ID()), "无效高版本不能阻止后续合法版本")
			after := read(1)
			observed, found := after.Snapshot.Member(remote.ID())
			require.True(t, found)
			assert.Equal(t, endpoint.StatusUp, observed.Status)
			assert.Equal(t, origin.View.Version().Get(remote.ID()), observed.Revision)
			assert.Equal(t, 2, after.Snapshot.Len())
		})
	}
}

func TestActor_GossipMemberLifecycleTCP(t *testing.T) {
	systems, refs, read := newMemberValidationSystems(t)
	origin := read(0)
	remote := origin.View.Members().Get(origin.ID)
	barrier := func() *gossipmessages.Pong {
		message, err := systems[0].Ask(refs[1], gossipmessages.NewPing("vivid-test", gossipmessages.GossipTypeCycleSYNC, remote, origin.View.Members(), origin.View.Version()), time.Second).Result()
		require.NoError(t, err)
		pong, ok := message.(*gossipmessages.Pong)
		require.True(t, ok)
		return pong
	}
	for _, protocol := range []string{"ping", "pong"} {
		for _, direct := range []bool{false, true} {
			for _, status := range []endpoint.Status{endpoint.StatusLeaving, endpoint.StatusRemoved} {
				name := protocol + "/" + map[bool]string{true: "direct", false: "relayed"}[direct] + "/" + status.String()
				t.Run(name, func(t *testing.T) {
					known := endpoint.NewInformation(remote.ActorRef.Clone(), remote.CreatedAt)
					known.Status = status
					members, version := origin.View.Members().Clone(), origin.View.Version().Clone()
					members.Upsert(known)
					version.Update(known.ID(), 2)
					systems[0].Tell(refs[1], gossipmessages.NewPing("vivid-test", gossipmessages.GossipTypeCycleSYNC, remote, members, version))
					require.Equal(t, status, barrier().MemberList.Get(known.ID()).Status)
					before := read(1).Snapshot
					send := func(info *endpoint.Information) {
						members = origin.View.Members().Clone()
						members.Upsert(info)
						sender := remote
						if direct {
							sender = info
						}
						if protocol == "ping" {
							systems[0].Tell(refs[1], gossipmessages.NewPing("vivid-test", gossipmessages.GossipTypeCycleSYNC, sender, members, version))
						} else {
							systems[0].Tell(refs[1], gossipmessages.NewPong("vivid-test", gossipmessages.GossipTypeCycleSYNC, sender, members, version))
						}
					}
					bad := known.Clone()
					bad.Status = endpoint.StatusJoining
					if status == endpoint.StatusRemoved {
						bad.Status = endpoint.StatusUp
					}
					version.Update(known.ID(), 100)
					send(bad)
					pong := barrier()
					assert.Equal(t, status, pong.MemberList.Get(known.ID()).Status)
					assert.Equal(t, uint64(2), pong.Version.Get(known.ID()))
					observed, exists := read(1).Snapshot.Member(known.ID())
					require.True(t, exists)
					assert.Equal(t, status, observed.Status)
					assert.Equal(t, uint64(2), observed.Revision, "回退记录不能进入公开成员快照")
					version = origin.View.Version().Clone()
					version.Update(known.ID(), 3)
					send(known)
					pong = barrier()
					assert.Equal(t, uint64(3), pong.Version.Get(known.ID()), "拒绝版本 100 后仍能接收合法版本 3")
					old, exists := before.Member(known.ID())
					require.True(t, exists)
					assert.Equal(t, status, old.Status)
					assert.Equal(t, uint64(2), old.Revision)
				})
			}
		}
	}
}
