package actor

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/clusterstore"
	"github.com/kercylan98/vivid/internal/virtual"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/require"
)

// BenchmarkVirtualRouteTCPAdmission 包含单节点一致性复核、本机 TCP 及入队回执，不含跨节点多数派 RTT。
func BenchmarkVirtualRouteTCPAdmission(b *testing.B) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(b, err)
	address := listener.Addr().String()
	require.NoError(b, listener.Close())
	var handled atomic.Int64
	control, ref := benchmarkRaftReadSystem(b, vivid.WithActorSystemRemoting(address))
	listener, err = net.Listen("tcp", "127.0.0.1:0")
	require.NoError(b, err)
	businessAddress := listener.Addr().String()
	require.NoError(b, listener.Close())
	business := NewSystem(vivid.WithActorSystemRemoting(businessAddress),
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError))),
		vivid.WithActorSystemVirtualActorProvider("worker", vivid.VirtualActorProviderFN(func(kind, name string) vivid.Actor {
			return vivid.ActorFN(func(ctx vivid.ActorContext) {
				virtualReply(ctx)
				if _, ok := ctx.Message().(*Ref); ok {
					handled.Add(1)
				}
			})
		})))
	owner, err := clusterstore.NewVirtualOwner(businessAddress)
	require.NoError(b, err)
	client, err := newVirtualOwnershipClient(business, clusterstore.RaftConfig{GroupID: "bench-read", NodeID: 1, Voters: []uint64{1}}, map[uint64]vivid.ActorRef{1: ref}, owner)
	require.NoError(b, err)
	require.NoError(b, business.virtualCoordinator.configureOwnership(client, 5*time.Second))
	require.NoError(b, business.Start())
	b.Cleanup(func() { require.NoError(b, business.Stop(5*time.Second)) })
	require.NoError(b, business.Ask(business.VirtualRef("worker", "one"), 1).Wait())
	ownership, exists, err := client.lookup(context.Background(), virtualKey{"worker", "one"})
	require.NoError(b, err)
	require.True(b, exists)
	caller, err := newVirtualOwnershipClient(control, clusterstore.RaftConfig{GroupID: "bench-read", NodeID: 1, Voters: []uint64{1}}, map[uint64]vivid.ActorRef{1: ref}, owner)
	require.NoError(b, err)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		status, err := caller.deliver(context.Background(), ownership, control.Ref(), control.Ref())
		if err != nil || status != virtual.DeliveryAccepted {
			b.Fatalf("投递失败: %v %v", status, err)
		}
	}
	b.StopTimer()
	// 同一业务队列的屏障确认已消费，避免将积压留到下一次采样。
	require.NoError(b, business.Ask(business.VirtualRef("worker", "one"), 1).Wait())
	require.EqualValues(b, b.N, handled.Load())
}
