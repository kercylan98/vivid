package clusterstore

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRaftRead_CurrentTermAndFreshQuorum(t *testing.T) {
	network := newRaftNetwork(t, 3)
	network.campaign(t, 1)
	// 只送达投票，不送达当选后的空操作复制。
	network.deliver(t, 0)
	network.deliver(t, 1)
	leader := network.nodes[1]
	require.Equal(t, RaftLeader, leader.role)
	_, err := leader.beginRead()
	require.ErrorIs(t, err, ErrRaftReadNotReady)
	network.drain(t)
	saves := network.disks[1].saves + network.disks[2].saves + network.disks[3].saves
	index := leader.hard.Commit
	for range 2 {
		out, err := leader.beginRead()
		require.NoError(t, err)
		require.Len(t, out, 2)
		require.False(t, leader.read.ready, "已有 Match 或 Leader 状态不能代替新确认")
		_, err = leader.beginRead()
		require.ErrorIs(t, err, ErrRaftReadPending)
		reply, err := network.nodes[2].Step(out[0])
		require.NoError(t, err)
		require.Len(t, reply, 1)
		_, err = leader.Step(reply[0])
		require.NoError(t, err)
		require.True(t, leader.read.ready)
		require.Equal(t, index, leader.read.index)
		leader.cancelRead(leader.read.request)
	}
	require.Equal(t, saves, network.disks[1].saves+network.disks[2].saves+network.disks[3].saves, "读取确认不得追加日志或刷盘")
	require.Equal(t, index, leader.hard.Commit)
}

func TestRaftRead_StaleDuplicateAndForeignResponses(t *testing.T) {
	network := newRaftNetwork(t, 5)
	network.campaign(t, 1)
	network.drain(t)
	leader := network.nodes[1]
	out, err := leader.beginRead()
	require.NoError(t, err)
	replies, err := network.nodes[2].Step(out[0])
	require.NoError(t, err)
	stale := replies[0]
	leader.cancelRead(leader.read.request)
	out, err = leader.beginRead()
	require.NoError(t, err)
	retry := leader.retryRead()
	require.Equal(t, out, retry, "重传保持同轮确认身份")
	_, err = leader.Step(stale)
	require.NoError(t, err)
	require.Len(t, leader.read.acknowledged, 1)
	replies, err = network.nodes[2].Step(out[0])
	require.NoError(t, err)
	valid := replies[0]
	for _, change := range []func(*RaftMessage){
		func(m *RaftMessage) { m.Index++ },
		func(m *RaftMessage) { m.Request++ },
		func(m *RaftMessage) { m.Reject = true },
	} {
		m := valid
		change(&m)
		_, err = leader.Step(m)
		require.NoError(t, err)
		require.Len(t, leader.read.acknowledged, 1)
	}
	for range 3 {
		_, err = leader.Step(valid)
		require.NoError(t, err)
	}
	require.Len(t, leader.read.acknowledged, 2)
	require.False(t, leader.read.ready, "重复同一成员不能凑成多数派")
	foreign := valid
	foreign.From = 99
	_, err = leader.Step(foreign)
	require.ErrorIs(t, err, ErrInvalidRaftMessage)
	foreign = valid
	foreign.Cluster[0] ^= 1
	_, err = leader.Step(foreign)
	require.ErrorIs(t, err, ErrInvalidRaftMessage)
	replies, err = network.nodes[3].Step(out[1])
	require.NoError(t, err)
	_, err = leader.Step(replies[0])
	require.NoError(t, err)
	require.True(t, leader.read.ready)
	require.EqualValues(t, 1, leader.peers[2].match, "读取应答不改变普通复制进度")
}

func TestRaftRead_PartitionAndLeadershipChange(t *testing.T) {
	network := newRaftNetwork(t, 3)
	network.campaign(t, 1)
	network.drain(t)
	old := network.nodes[1]
	out, err := old.beginRead()
	require.NoError(t, err)
	oldProbe := out[0]
	network.blocked[1] = true
	network.queue = append(network.queue, out...)
	network.drain(t)
	require.False(t, old.read.ready)
	network.campaign(t, 2)
	network.drain(t)
	require.Equal(t, RaftLeader, network.nodes[2].role)
	// 新 Leader 的多数派任期已推进，拒绝旧读，即使旧 Leader 尚未知情。
	replies, err := network.nodes[2].Step(oldProbe)
	require.NoError(t, err)
	require.True(t, replies[0].Reject)
	_, err = old.Step(replies[0])
	require.NoError(t, err)
	require.Nil(t, old.read)
	_, err = old.beginRead()
	require.ErrorIs(t, err, ErrRaftNotLeader)
}

func TestRaftRead_SingleNodeCompactionAndFailures(t *testing.T) {
	network := newRaftNetwork(t, 1)
	network.campaign(t, 1)
	leader := network.nodes[1]
	store, err := replayRaftStore(leader, &Store{}, nil)
	require.NoError(t, err)
	require.NoError(t, leader.Compact(store.Snapshot()))
	out, err := leader.beginRead()
	require.NoError(t, err)
	require.Empty(t, out)
	require.True(t, leader.read.ready)
	leader.cancelRead(leader.read.request + 1)
	require.NotNil(t, leader.read, "旧取消不能清除当前读取")
	leader.cancelRead(leader.read.request)
	leader.sequence = ^uint64(0)
	_, err = leader.beginRead()
	require.ErrorIs(t, err, ErrRaftExhausted)
	require.Nil(t, leader.read)
	leader.failed = io.ErrUnexpectedEOF
	_, err = leader.beginRead()
	require.ErrorIs(t, err, ErrRaftNodeFailed)
}

func TestRaftRead_HigherTermPersistsBeforeReply(t *testing.T) {
	network := newRaftNetwork(t, 3)
	network.campaign(t, 1)
	network.drain(t)
	out, err := network.nodes[1].beginRead()
	require.NoError(t, err)
	probe := out[0]
	probe.Term++
	network.disks[2].failure = io.ErrShortWrite
	replies, err := network.nodes[2].Step(probe)
	require.ErrorIs(t, err, io.ErrShortWrite)
	require.Empty(t, replies, "任期落盘失败不能发出读取确认")
}
