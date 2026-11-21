package actor

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/clusterstore"
	"github.com/kercylan98/vivid/internal/mailbox"
	"github.com/kercylan98/vivid/internal/remoting"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/require"
)

type ownedPayload struct{ Data []byte }

func newOwnedSystem(t *testing.T, encode func(*vivid.Writer, any) error, options ...vivid.ActorSystemOption) *System {
	t.Helper()
	if encode == nil {
		encode = func(w *vivid.Writer, message any) error {
			w.WriteBytes(message.(*ownedPayload).Data)
			return w.Err()
		}
	}
	configured := []vivid.ActorSystemOption{
		vivid.WithActorSystemRemoting(admissionSystemAddress(t)),
		vivid.WithActorSystemMessageRegister(vivid.MessageRegisterFN(func(registry vivid.MessageRegistry) {
			registry.RegisterMessageWithEncoderAndDecoder("owned-payload", new(ownedPayload), vivid.MessageEncoderFN(encode), vivid.MessageDecoderFN(func(r *vivid.Reader, message any) error {
				return r.ReadBytes(&message.(*ownedPayload).Data)
			}))
		})),
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
	}
	system := NewSystem(append(configured, options...)...)
	require.NoError(t, system.Start())
	t.Cleanup(func() {
		err := system.Stop(2 * time.Second)
		require.True(t, err == nil || errors.Is(err, vivid.ErrorActorSystemAlreadyStopped), "%v", err)
	})
	return system
}

func ownedPacketTarget(t *testing.T, system *System) (vivid.ActorRef, <-chan []byte) {
	t.Helper()
	packets := make(chan []byte, 4)
	ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		if message, ok := ctx.Message().(*ownedPayload); ok {
			packets <- message.Data
		}
	}), vivid.WithActorName("snapshot-receiver"))
	require.NoError(t, err)
	return ref, packets
}

func TestSystem_RemotingOwnedSnapshot(t *testing.T) {
	// 固定已有提交和 Leader 资格；本测试验证生产 Remoting 的传输与释放，不验证自动选举。
	config := clusterstore.RaftConfig{GroupID: "owned-snapshot", NodeID: 1, Voters: []uint64{1, 2, 3}}
	source, err := clusterstore.CreateRaftStorage(filepath.Join(t.TempDir(), "source.wal"), config)
	require.NoError(t, err)
	defer source.Close()
	var state clusterstore.Store
	var entries []clusterstore.RaftEntry
	for i := uint64(1); i <= 6; i++ {
		command := clusterstore.Command{}
		if i > 1 {
			command = clusterstore.Command{Operation: clusterstore.Put, Key: fmt.Sprint(i), Value: bytes.Repeat([]byte{byte(i)}, 1<<20)}
		}
		_, err := state.Apply(i, command)
		require.NoError(t, err)
		entries = append(entries, clusterstore.RaftEntry{Index: i, Term: 1})
	}
	require.NoError(t, source.Save(clusterstore.RaftUpdate{HardState: clusterstore.RaftHardState{Term: 1, Vote: 1, Commit: 6}, Entries: entries}))
	require.NoError(t, source.Compact(state.Snapshot()))
	config.NodeID = 3
	targetPath := filepath.Join(t.TempDir(), "target.wal")
	target, err := clusterstore.CreateRaftStorage(targetPath, config)
	require.NoError(t, err)
	defer target.Close()
	leader, err := clusterstore.NewRaftNode(source)
	require.NoError(t, err)
	follower, err := clusterstore.NewRaftNode(target)
	require.NoError(t, err)
	requests, err := leader.Campaign()
	require.NoError(t, err)
	requests, err = leader.Step(clusterstore.RaftMessage{Cluster: requests[0].Cluster, Type: clusterstore.RaftVoteResponse, From: 2, To: 1, Term: 2})
	require.NoError(t, err)
	drain := func(queue []clusterstore.RaftMessage) {
		for count := 0; len(queue) > 0; count++ {
			require.Less(t, count, 100)
			message := queue[0]
			queue = queue[1:]
			if message.To == 2 {
				continue
			}
			node := leader
			if message.To == 3 {
				node = follower
			}
			out, err := node.Step(message)
			require.NoError(t, err)
			queue = append(queue, out...)
		}
	}
	drain(requests)
	budget, err := clusterstore.NewSnapshotTransferBudget(clusterstore.SnapshotTransferLimits{})
	require.NoError(t, err)
	sender, err := clusterstore.NewRaftSnapshotTransfer(leader, budget)
	require.NoError(t, err)
	defer sender.Close()
	receiver, err := clusterstore.NewRaftSnapshotTransfer(follower, budget)
	require.NoError(t, err)
	defer receiver.Close()
	left, right := newOwnedSystem(t, nil), newOwnedSystem(t, nil)
	leftRef, leftPackets := ownedPacketTarget(t, left)
	rightRef, rightPackets := ownedPacketTarget(t, right)
	send := func(system *System, from, to vivid.ActorRef, frame *clusterstore.SnapshotFrame, packets <-chan []byte) clusterstore.SnapshotPacket {
		completed := make(chan bool, 2)
		envelop := mailbox.NewEnvelop(false, from, to, &ownedPayload{Data: frame.Bytes})
		system.ResolveMailbox(to).Enqueue(remoting.NewOwnedEnvelop(envelop, func(written bool) {
			frame.Release()
			completed <- written
		}))
		require.True(t, receiveRetryResult(t, completed))
		require.Nil(t, frame.Bytes)
		packet, err := clusterstore.DecodeSnapshotPacket(receiveRetryResult(t, packets))
		require.NoError(t, err)
		require.Empty(t, completed)
		return packet
	}
	frame, err := sender.Send(3, time.Now())
	require.NoError(t, err)
	for count := 0; frame != nil; count++ {
		require.Less(t, count, 30)
		packet := send(left, leftRef, rightRef, frame, rightPackets)
		require.Greater(t, packet.ID.Size, uint64(4<<20))
		ack, _, err := receiver.Step(packet, time.Now())
		require.NoError(t, err)
		response := send(right, rightRef, leftRef, ack, leftPackets)
		var out []clusterstore.RaftMessage
		frame, out, err = sender.Step(response, time.Now())
		require.NoError(t, err)
		drain(out)
	}
	require.Equal(t, clusterstore.SnapshotTransferUsage{}, budget.Usage())
	receiver.Close()
	require.NoError(t, target.Close())
	restored, err := clusterstore.OpenRaftStorage(targetPath, config)
	require.NoError(t, err)
	defer restored.Close()
	snapshot, err := restored.Snapshot()
	require.NoError(t, err)
	actual, err := clusterstore.Restore(snapshot.Data)
	require.NoError(t, err)
	for i := uint64(2); i <= 6; i++ {
		record, exists := actual.Get(fmt.Sprint(i))
		require.True(t, exists)
		require.Equal(t, bytes.Repeat([]byte{byte(i)}, 1<<20), record.Value)
	}
	hard, err := restored.State()
	require.NoError(t, err)
	require.EqualValues(t, 7, hard.Commit)
}

func TestSystem_RemotingOwnedAdmissionStopDuringEncoding(t *testing.T) {
	entered, resume := make(chan struct{}), make(chan struct{})
	var release sync.Once
	defer release.Do(func() { close(resume) })
	source := newOwnedSystem(t, func(w *vivid.Writer, message any) error {
		close(entered)
		<-resume
		w.WriteBytes(message.(*ownedPayload).Data)
		return w.Err()
	})
	peer := newOwnedSystem(t, nil)
	ref, _ := ownedPacketTarget(t, peer)
	envelop := mailbox.NewEnvelop(false, source.Ref(), ref, &ownedPayload{Data: []byte("snapshot")})
	completed := make(chan bool, 2)
	source.ResolveMailbox(ref).Enqueue(remoting.NewOwnedEnvelop(envelop, func(written bool) { completed <- written }))
	receiveRetryResult(t, entered)
	// 直接结束真实系统共享准入，模拟关闭已决定放弃但 writer 尚未返回的窗口。
	source.remotingAdmission.Stop()
	require.Empty(t, completed)
	release.Do(func() { close(resume) })
	require.False(t, receiveRetryResult(t, completed))
	require.Empty(t, completed)
}

func TestSystem_RemotingOwnedReconnect(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	address := listener.Addr().String()
	refused := make(chan struct{})
	go func() {
		defer close(refused)
		conn, err := listener.Accept()
		if err == nil {
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(time.Second))
			_ = new(remoting.Handshake).Wait(conn)
		}
	}()
	source := newOwnedSystem(t, nil, vivid.WithActorSystemRemotingOption(vivid.WithActorSystemRemotingReconnect(3, 100*time.Millisecond, 100*time.Millisecond, 1, false)))
	ref, err := source.CreateRef(address, "/snapshot-receiver")
	require.NoError(t, err)
	completed := make(chan bool, 2)
	envelop := mailbox.NewEnvelop(false, source.Ref(), ref, &ownedPayload{Data: []byte("retained across reconnect")})
	source.ResolveMailbox(ref).Enqueue(remoting.NewOwnedEnvelop(envelop, func(written bool) { completed <- written }))
	receiveRetryResult(t, refused)
	require.Empty(t, completed, "临时连接失败不能释放还会重试的负载")
	require.NoError(t, listener.Close())
	peer := newOwnedSystem(t, nil, vivid.WithActorSystemRemoting(address))
	_, packets := ownedPacketTarget(t, peer)
	require.True(t, receiveRetryResult(t, completed))
	require.Equal(t, []byte("retained across reconnect"), receiveRetryResult(t, packets))
	require.Empty(t, completed)
}

func TestSystem_RemotingOwnedEncodingFailure(t *testing.T) {
	source := newOwnedSystem(t, func(*vivid.Writer, any) error { return errors.New("invalid payload") })
	peer := newOwnedSystem(t, nil)
	ref, packets := ownedPacketTarget(t, peer)
	completed := make(chan bool, 2)
	envelop := mailbox.NewEnvelop(false, source.Ref(), ref, &ownedPayload{Data: []byte("snapshot")})
	source.ResolveMailbox(ref).Enqueue(remoting.NewOwnedEnvelop(envelop, func(written bool) { completed <- written }))
	require.False(t, receiveRetryResult(t, completed))
	require.Empty(t, packets)
	require.Empty(t, completed)
}

func TestSystem_RemotingOwnedStopBeforeDelivery(t *testing.T) {
	source := newOwnedSystem(t, nil)
	value, exists := source.actorContexts.Load(source.remotingRef.GetPath())
	require.True(t, exists)
	remoteContext := value.(*Context)
	remoteContext.mailbox.Pause()
	defer remoteContext.mailbox.Resume()
	ref, err := source.CreateRef("127.0.0.1:1", "/snapshot-receiver")
	require.NoError(t, err)
	completed := make(chan bool, 2)
	envelop := mailbox.NewEnvelop(false, source.Ref(), ref, &ownedPayload{Data: []byte("queued")})
	source.ResolveMailbox(ref).Enqueue(remoting.NewOwnedEnvelop(envelop, func(written bool) { completed <- written }))
	require.NoError(t, source.Stop(2*time.Second))
	require.False(t, receiveRetryResult(t, completed))
	require.Empty(t, completed)
}
