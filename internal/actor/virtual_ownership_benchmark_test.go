package actor

import (
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
	control, ref := benchmarkRaftReadSystem(b)
	owner, err := clusterstore.NewVirtualOwner("owner.example:9000")
	require.NoError(b, err)
	client, err := newVirtualOwnershipClient(control, ref, owner)
	require.NoError(b, err)
	system := NewSystem(vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))),
		vivid.WithActorSystemVirtualActorProvider("worker", vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
			return vivid.ActorFN(virtualReply)
		})))
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
