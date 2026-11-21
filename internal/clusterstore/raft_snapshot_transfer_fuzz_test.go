package clusterstore

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func FuzzSnapshotTransfer_Network(f *testing.F) {
	f.Add([]byte{0, 1, 0, 3, 4, 0, 1, 1})
	f.Add([]byte{0, 2, 0, 6, 1, 4, 5, 0, 1})
	f.Add([]byte("1202")) // 确认任期损坏后必须先被编码层拒绝
	f.Fuzz(func(t *testing.T, operations []byte) {
		if len(operations) > 64 {
			return
		}
		network, sender, receiver, budget := transferFixture(t, 0, SnapshotTransferLimits{Timeout: time.Second})
		clear(network.blocked)
		now := time.Now()
		var queue []*SnapshotFrame
		defer func() {
			for _, frame := range queue {
				frame.Release()
			}
		}()
		allowed := func(err error) {
			t.Helper()
			if err != nil {
				require.True(t, errors.Is(err, ErrSnapshotTransferBusy) || errors.Is(err, ErrSnapshotTransferCapacity) || errors.Is(err, ErrSnapshotTransferStale) || errors.Is(err, ErrInvalidSnapshotPacket), "%v", err)
			}
		}
		deliver := func(frame *SnapshotFrame, duplicate bool) {
			t.Helper()
			packet, err := DecodeSnapshotPacket(frame.Bytes)
			if err != nil {
				allowed(err)
				frame.Release()
				return
			}
			target := receiver
			if packet.Type == SnapshotChunkAck {
				target = sender
			}
			out, messages, err := target.Step(packet, now)
			allowed(err)
			if out != nil {
				queue = append(queue, out)
			}
			network.queue = append(network.queue, messages...)
			if duplicate {
				out, messages, err = target.Step(packet, now)
				allowed(err)
				if out != nil {
					queue = append(queue, out)
				}
				network.queue = append(network.queue, messages...)
			}
			frame.Release()
			network.drain(t)
		}
		for _, operation := range operations {
			switch operation % 7 {
			case 0:
				if sender.node.peers[3].snapshot {
					frame, err := sender.Send(3, now)
					allowed(err)
					if frame != nil {
						queue = append(queue, frame)
					}
				}
			case 1, 2, 3:
				if len(queue) > 0 {
					index := int(operation/7) % len(queue)
					frame := queue[index]
					queue = append(queue[:index], queue[index+1:]...)
					if operation%7 == 2 {
						frame.Release()
					} else {
						deliver(frame, operation%7 == 3)
					}
				}
			case 4:
				now = now.Add(time.Second)
				require.NoError(t, sender.Expire(now))
				require.NoError(t, receiver.Expire(now))
			case 5:
				sender.Cancel(3)
				receiver.Cancel(1)
			case 6:
				if len(queue) > 0 {
					frame := queue[int(operation/7)%len(queue)]
					frame.Bytes[len(frame.Bytes)-1] ^= operation | 1
				}
			}
			usage := budget.Usage()
			require.GreaterOrEqual(t, usage.Sessions, 0)
			require.LessOrEqual(t, usage.Sessions, budget.limits.MaxSessions)
			require.LessOrEqual(t, usage.SnapshotBytes, budget.limits.MaxSnapshotBytes)
			require.LessOrEqual(t, usage.InFlightBytes, budget.limits.MaxInFlightBytes)
			network.check(t)
		}
		for _, frame := range queue {
			frame.Release()
		}
		queue = nil
		sender.Cancel(3)
		receiver.Cancel(1)
		// 停止故障后重新建立会话并完全送达，旧拒绝水位不能阻断新请求。
		if sender.node.peers[3].snapshot {
			frame, err := sender.Send(3, now)
			require.NoError(t, err)
			queue = append(queue, frame)
			for i := 0; len(queue) > 0; i++ {
				require.Less(t, i, 10)
				frame := queue[0]
				queue = queue[1:]
				deliver(frame, false)
			}
		}
		require.EqualValues(t, 2, network.nodes[3].hard.Commit)
		sender.Close()
		receiver.Close()
		require.Equal(t, SnapshotTransferUsage{}, budget.Usage())
	})
}
