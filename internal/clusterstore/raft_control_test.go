package clusterstore

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/serialization"
	"github.com/stretchr/testify/require"
)

func TestRaftControl_CodecAndErrors(t *testing.T) {
	group, err := RaftControlGroup(RaftConfig{GroupID: "control", NodeID: 1, Voters: []uint64{1, 2, 3}})
	require.NoError(t, err)
	reordered, err := RaftControlGroup(RaftConfig{GroupID: "control", NodeID: 2, Voters: []uint64{3, 2, 1}})
	require.NoError(t, err)
	require.Equal(t, group, reordered)
	request := &RaftControlRequest{Group: group, Node: 1, Operation: RaftControlExecute, Timeout: time.Second, Command: Command{Operation: Put, Key: "owner", Value: []byte("node-a")}}
	w := serialization.NewWriter()
	require.NoError(t, request.Encode(w, request))
	data := w.Bytes()
	var decoded RaftControlRequest
	require.NoError(t, decoded.Decode(serialization.NewReader(data), &decoded))
	require.Equal(t, *request, decoded)
	decoded.Command.Value[0] = 'x'
	require.Equal(t, []byte("node-a"), request.Command.Value)
	for size := range len(data) {
		previous := *request
		require.Error(t, decoded.Decode(serialization.NewReader(data[:size]), &previous))
		require.Equal(t, *request, previous, "失败解码不可部分覆盖目标")
	}
	for code := controlOK; code <= controlLogCapacity; code++ {
		response := &RaftControlResponse{Group: group, Node: 1, Operation: RaftControlExecute, Role: RaftLeader, code: code}
		if code == controlOK {
			response.Index, response.Term, response.Revision, response.Applied = 3, 1, 3, true
		}
		w := serialization.NewWriter()
		require.NoError(t, response.Encode(w, response))
		var actual RaftControlResponse
		require.NoError(t, actual.Decode(serialization.NewReader(w.Bytes()), &actual))
		require.Equal(t, *response, actual)
		require.ErrorIs(t, actual.Execution().Err, code.err())
		for size := range len(w.Bytes()) {
			require.Error(t, actual.Decode(serialization.NewReader(w.Bytes()[:size]), &actual))
		}
		invalid := bytes.Clone(w.Bytes())
		invalid[0] = 2
		require.ErrorIs(t, actual.Decode(serialization.NewReader(invalid), &actual), ErrInvalidRaftControl)
	}
	require.Equal(t, controlUnknown, controlError(errors.Join(ErrRaftOutcomeUnknown, ErrRaftNotLeader)))
	require.Equal(t, controlStateCapacity, controlError(ErrCapacity))
	var invalid RaftControlRequest
	require.ErrorIs(t, invalid.Encode(serialization.NewWriter(), &invalid), ErrInvalidRaftControl)
	request.Command.Value = make([]byte, maxValueBytes+1)
	require.ErrorIs(t, request.Encode(serialization.NewWriter(), request), ErrInvalidRaftControl)
	response := &RaftControlResponse{Group: group, Node: 1, Operation: RaftControlGet, Index: 3, Term: 1, Revision: 2, Exists: true, Value: []byte("value")}
	w = serialization.NewWriter()
	require.NoError(t, response.Encode(w, response))
	var actual RaftControlResponse
	require.NoError(t, actual.Decode(serialization.NewReader(w.Bytes()), &actual))
	require.Equal(t, response.Record(), actual.Record())
	actual.Value[0] = 'x'
	require.Equal(t, []byte("value"), response.Value)
	require.ErrorIs(t, response.Execution().Err, ErrInvalidRaftControl)
	require.False(t, (&RaftControlResponse{Group: group, Node: 1, Operation: RaftControlExecute, Index: 3, code: controlNotLeader}).valid(), "已经提案的拒绝不可表示为可重发")
	response.Operation = RaftControlLocate
	require.False(t, response.valid(), "定位回执不可携带读取负载")
	request.Command.Value = make([]byte, maxValueBytes)
	w = serialization.NewWriter()
	require.NoError(t, request.Encode(w, request))
	var maximum RaftControlRequest
	require.NoError(t, maximum.Decode(serialization.NewReader(w.Bytes()), &maximum))
	require.Len(t, maximum.Command.Value, maxValueBytes)
}

type controlTestContext struct {
	vivid.ActorContext
	sender  vivid.ActorRef
	replies []*RaftControlResponse
}

func (c *controlTestContext) Sender() vivid.ActorRef      { return c.sender }
func (c *controlTestContext) Reply(message vivid.Message) { c.Tell(c.sender, message) }
func (c *controlTestContext) Tell(_ vivid.ActorRef, message vivid.Message) {
	c.replies = append(c.replies, message.(*RaftControlResponse))
}

func TestRaftControl_AdmissionAndExecution(t *testing.T) {
	driver, original := newExecutionDriver(t)
	driver.opts.Config = driver.node.config
	driver.opts.MaxPendingReads = 3
	ctx := &controlTestContext{sender: original.sender}
	request := &RaftControlRequest{Group: driver.node.cluster, Node: driver.node.config.NodeID, Operation: RaftControlExecute, Command: Command{Operation: Put, Key: "owner", Value: []byte("a")}}
	wrong := *request
	wrong.Group[0] ^= 1
	driver.control(ctx, &wrong, time.Now())
	require.ErrorIs(t, ctx.replies[0].Err(), ErrInvalidRaftControl)
	require.Empty(t, driver.executions)
	ctx.replies = nil
	driver.control(ctx, request, time.Now())
	require.Empty(t, ctx.replies, "网络入口不能提前回执未应用结果")
	require.Len(t, driver.executions, 1)
	driver.apply(ctx)
	require.True(t, ctx.replies[0].Execution().Result.Applied)
	index := ctx.replies[0].Index
	driver.control(ctx, request, time.Now())
	driver.apply(ctx)
	require.NoError(t, ctx.replies[1].Err())
	require.False(t, ctx.replies[1].Applied, "CAS 冲突也必须来自实际执行结果")
	ctx.replies = nil
	read := &RaftControlRequest{Group: request.Group, Node: request.Node, Operation: RaftControlGet, Key: "owner"}
	driver.control(ctx, read, time.Now())
	driver.checkReads(ctx)
	require.Len(t, ctx.replies, 1)
	require.Equal(t, Record{Revision: index, Value: []byte("a")}, ctx.replies[0].Record().Record)
	driver.node.follow(0)
	driver.control(ctx, request, time.Now())
	require.ErrorIs(t, ctx.replies[1].Err(), ErrRaftNotLeader)
	require.Zero(t, ctx.replies[1].Index, "非 Leader 拒绝必须发生在提案之前")
}

func FuzzRaftControl_Decode(f *testing.F) {
	group, _ := RaftControlGroup(RaftConfig{GroupID: "fuzz", NodeID: 1, Voters: []uint64{1}})
	request := &RaftControlRequest{Group: group, Node: 1, Operation: RaftControlGet, Key: "owner"}
	w := serialization.NewWriter()
	_ = request.Encode(w, request)
	f.Add(w.Bytes())
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 16<<10 {
			return
		}
		var request RaftControlRequest
		if request.Decode(serialization.NewReader(data), &request) == nil {
			require.True(t, request.valid())
		}
		var response RaftControlResponse
		if response.Decode(serialization.NewReader(data), &response) == nil {
			require.True(t, response.valid())
		}
	})
}
