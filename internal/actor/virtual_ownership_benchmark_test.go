package actor

import (
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/clusterstore"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/require"
)

// BenchmarkOwnedVirtualActivate 包含单节点 WAL 刷盘、两次一致性复核和首次业务应答，不含跨节点网络。
func BenchmarkOwnedVirtualActivate(b *testing.B) {
	benchmarkOwnedVirtualActivate(b, false)
}

// BenchmarkRemoteOwnedVirtualActivate 额外包含独立系统之间的真实本机 TCP 控制请求。
func BenchmarkRemoteOwnedVirtualActivate(b *testing.B) {
	benchmarkOwnedVirtualActivate(b, true)
}

func benchmarkOwnedVirtualActivate(b *testing.B, remote bool) {
	b.Helper()
	address := func() string {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(b, err)
		value := listener.Addr().String()
		require.NoError(b, listener.Close())
		return value
	}
	var options []vivid.ActorSystemOption
	if remote {
		options = append(options, vivid.WithActorSystemRemoting(address()))
	}
	control, ref := benchmarkRaftReadSystem(b, options...)
	options = []vivid.ActorSystemOption{vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))),
		vivid.WithActorSystemVirtualActorProvider("worker", vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
			return vivid.ActorFN(virtualReply)
		}))}
	ownerAddress := "owner.example:9000"
	if remote {
		ownerAddress = address()
		options = append(options, vivid.WithActorSystemRemoting(ownerAddress))
	}
	system := NewSystem(options...)
	caller := control
	if remote {
		caller = system
	}
	owner, err := clusterstore.NewVirtualOwner(ownerAddress)
	require.NoError(b, err)
	client, err := newVirtualOwnershipClient(caller, clusterstore.RaftConfig{GroupID: "bench-read", NodeID: 1, Voters: []uint64{1}}, map[uint64]vivid.ActorRef{1: ref}, owner)
	require.NoError(b, err)
	require.NoError(b, system.virtualCoordinator.configureOwnership(client, 30*time.Second))
	require.NoError(b, system.Start())
	b.Cleanup(func() { require.NoError(b, system.Stop(30*time.Second)) })
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		if err := system.Ask(system.VirtualRef("worker", strconv.Itoa(i)), 1, 30*time.Second).Wait(); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
}
