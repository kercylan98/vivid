// 集群消息在 remoting 层使用内部序列化，通过 internal/messages 注册并实现 reader/writer。
// NodeState、ClusterView 使用二进制格式读写，不依赖应用 Codec，避免 gob 解码 interface{} 失败。

package cluster

import (
	"time"

	"github.com/kercylan98/vivid/internal/messages"
)

func init() {
	registerClusterInternalMessages()
}

func registerClusterInternalMessages() {
	messages.RegisterInternalMessage[*JoinRequest](
		"ClusterInternalMessageForJoinRequest", clusterJoinRequestReader, clusterJoinRequestWriter)
	messages.RegisterInternalMessage[*JoinResponse](
		"ClusterInternalMessageForJoinResponse", clusterJoinResponseReader, clusterJoinResponseWriter)
	messages.RegisterInternalMessage[*GossipMessage](
		"ClusterInternalMessageForGossip", clusterGossipReader, clusterGossipWriter)
	messages.RegisterInternalMessage[*GossipTick](
		"ClusterInternalMessageForGossipTick", clusterNoopReader, clusterNoopWriter)
	messages.RegisterInternalMessage[*GossipCrossDCTick](
		"ClusterInternalMessageForGossipCrossDCTick", clusterNoopReader, clusterNoopWriter)
	messages.RegisterInternalMessage[*FailureDetectionTick](
		"ClusterInternalMessageForFailureDetectionTick", clusterNoopReader, clusterNoopWriter)
	messages.RegisterInternalMessage[*GetViewRequest](
		"ClusterInternalMessageForGetViewRequest", clusterNoopReader, clusterNoopWriter)
	messages.RegisterInternalMessage[*GetViewResponse](
		"ClusterInternalMessageForGetViewResponse", clusterGetViewResponseReader, clusterGetViewResponseWriter)
	messages.RegisterInternalMessage[*LeaveRequest](
		"ClusterInternalMessageForLeaveRequest", clusterNoopReader, clusterNoopWriter)
	messages.RegisterInternalMessage[*LeaveAck](
		"ClusterInternalMessageForLeaveAck", clusterNoopReader, clusterNoopWriter)
	messages.RegisterInternalMessage[*ExitingReady](
		"ClusterInternalMessageForExitingReady", clusterNoopReader, clusterNoopWriter)
	messages.RegisterInternalMessage[*LeaveBroadcastRound](
		"ClusterInternalMessageForLeaveBroadcastRound", clusterLeaveBroadcastRoundReader, clusterLeaveBroadcastRoundWriter)
	messages.RegisterInternalMessage[*JoinRetryTick](
		"ClusterInternalMessageForJoinRetryTick", clusterJoinRetryTickReader, clusterJoinRetryTickWriter)
	messages.RegisterInternalMessage[*ForceMemberDown](
		"ClusterInternalMessageForForceMemberDown", clusterForceMemberDownReader, clusterForceMemberDownWriter)
	messages.RegisterInternalMessage[*TriggerViewBroadcast](
		"ClusterInternalMessageForTriggerViewBroadcast", clusterTriggerViewBroadcastReader, clusterTriggerViewBroadcastWriter)
}

func clusterNoopReader(message any, reader *messages.Reader, codec messages.Codec) error { return nil }
func clusterNoopWriter(message any, writer *messages.Writer, codec messages.Codec) error { return nil }

// writeNodeState 将 NodeState 按二进制格式写入，nil 写长度 0。
func writeNodeState(w *messages.Writer, n *NodeState) error {
	if n == nil {
		w.WriteUint32(0)
		return w.Err()
	}
	// 用长度 1 标记“有内容”，避免与“空但非 nil”的 NodeState 歧义
	w.WriteUint32(1)
	writerWriteNodeStateBody(w, n)
	return w.Err()
}

func writerWriteNodeStateBody(w *messages.Writer, n *NodeState) {
	w.WriteString(n.ID).WriteString(n.ClusterName).WriteString(n.Address)
	w.WriteInt32(int32(n.Generation)).WriteUint64(n.Version).WriteInt64(n.Timestamp).WriteUint64(n.SeqNo)
	w.WriteInt32(int32(n.Status)).WriteBool(n.Unreachable).WriteInt64(n.LastSeen).WriteUint64(n.LogicalClock)
	writeMapStringString(w, n.Metadata)
	writeMapStringString(w, n.Labels)
	w.WriteUint32(n.Checksum)
}

func writeMapStringString(w *messages.Writer, m map[string]string) {
	if m == nil {
		w.WriteUint32(0)
		return
	}
	w.WriteUint32(uint32(len(m)))
	for k, v := range m {
		w.WriteString(k).WriteString(v)
	}
}

func readNodeState(r *messages.Reader) (*NodeState, error) {
	n, err := r.ReadUint32()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, nil
	}
	return readerReadNodeStateBody(r)
}

func readerReadNodeStateBody(r *messages.Reader) (*NodeState, error) {
	id, _ := r.ReadString()
	clusterName, _ := r.ReadString()
	address, _ := r.ReadString()
	gen, _ := r.ReadInt32()
	version, _ := r.ReadUint64()
	timestamp, _ := r.ReadInt64()
	seqNo, _ := r.ReadUint64()
	status, _ := r.ReadInt32()
	unreachable, _ := r.ReadBool()
	lastSeen, _ := r.ReadInt64()
	logicalClock, _ := r.ReadUint64()
	metadata := readMapStringString(r)
	labels := readMapStringString(r)
	checksum, _ := r.ReadUint32()
	if r.Error() != nil {
		return nil, r.Error()
	}
	return &NodeState{
		ID: id, ClusterName: clusterName, Address: address,
		Generation: int(gen), Version: version, Timestamp: timestamp, SeqNo: seqNo,
		Status: MemberStatus(status), Unreachable: unreachable, LastSeen: lastSeen, LogicalClock: logicalClock,
		Metadata: metadata, Labels: labels, Checksum: checksum,
	}, nil
}

func readMapStringString(r *messages.Reader) map[string]string {
	n, err := r.ReadUint32()
	if err != nil || n == 0 {
		return nil
	}
	m := make(map[string]string, n)
	for i := uint32(0); i < n; i++ {
		k, _ := r.ReadString()
		v, _ := r.ReadString()
		m[k] = v
	}
	return m
}

// writeClusterView 将 ClusterView 按二进制格式写入，nil 写长度 0。
func writeClusterView(w *messages.Writer, v *ClusterView) error {
	if v == nil {
		w.WriteUint32(0)
		return w.Err()
	}
	w.WriteUint32(1)
	w.WriteString(v.ViewID).WriteInt64(v.Epoch).WriteInt64(v.Timestamp)
	w.WriteUint32(uint32(len(v.Members)))
	for id, state := range v.Members {
		w.WriteString(id)
		if state != nil {
			w.WriteUint8(1)
			writerWriteNodeStateBody(w, state)
		} else {
			w.WriteUint8(0)
		}
	}
	w.WriteInt32(int32(v.HealthyCount)).WriteInt32(int32(v.UnhealthyCount)).WriteInt32(int32(v.QuorumSize))
	_ = WriteVersionVector(w, v.VersionVector)
	w.WriteUint16(v.ProtocolVersion).WriteInt32(int32(v.MaxVersionVectorEntries))
	return w.Err()
}

func readClusterView(r *messages.Reader) (*ClusterView, error) {
	n, err := r.ReadUint32()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, nil
	}
	viewID, _ := r.ReadString()
	epoch, _ := r.ReadInt64()
	timestamp, _ := r.ReadInt64()
	memLen, _ := r.ReadUint32()
	members := make(map[string]*NodeState, memLen)
	for i := uint32(0); i < memLen; i++ {
		id, _ := r.ReadString()
		has, _ := r.ReadUint8()
		if has != 0 {
			state, _ := readerReadNodeStateBody(r)
			members[id] = state
		}
	}
	healthy, _ := r.ReadInt32()
	unhealthy, _ := r.ReadInt32()
	quorum, _ := r.ReadInt32()
	vv, _ := ReadVersionVector(r)
	protoVer, _ := r.ReadUint16()
	maxVV, _ := r.ReadInt32()
	if r.Error() != nil {
		return nil, r.Error()
	}
	return &ClusterView{
		ViewID: viewID, Epoch: epoch, Timestamp: timestamp, Members: members,
		HealthyCount: int(healthy), UnhealthyCount: int(unhealthy), QuorumSize: int(quorum),
		VersionVector: vv, ProtocolVersion: protoVer, MaxVersionVectorEntries: int(maxVV),
	}, nil
}

func clusterJoinRequestReader(message any, reader *messages.Reader, codec messages.Codec) error {
	m := message.(*JoinRequest)
	ns, err := readNodeState(reader)
	if err != nil {
		return err
	}
	m.NodeState = ns
	s, err := reader.ReadString()
	if err != nil {
		return err
	}
	m.AuthToken = s
	return nil
}

func clusterJoinRequestWriter(message any, writer *messages.Writer, codec messages.Codec) error {
	m := message.(*JoinRequest)
	if err := writeNodeState(writer, m.NodeState); err != nil {
		return err
	}
	writer.WriteString(m.AuthToken)
	return writer.Err()
}

func clusterJoinResponseReader(message any, reader *messages.Reader, codec messages.Codec) error {
	m := message.(*JoinResponse)
	v, err := readClusterView(reader)
	if err != nil {
		return err
	}
	m.View = v
	return nil
}

func clusterJoinResponseWriter(message any, writer *messages.Writer, codec messages.Codec) error {
	m := message.(*JoinResponse)
	return writeClusterView(writer, m.View)
}

func clusterGossipReader(message any, reader *messages.Reader, codec messages.Codec) error {
	m := message.(*GossipMessage)
	v, err := readClusterView(reader)
	if err != nil {
		return err
	}
	m.View = v
	return nil
}

func clusterGossipWriter(message any, writer *messages.Writer, codec messages.Codec) error {
	m := message.(*GossipMessage)
	return writeClusterView(writer, m.View)
}

func clusterGetViewResponseReader(message any, reader *messages.Reader, codec messages.Codec) error {
	m := message.(*GetViewResponse)
	v, err := readClusterView(reader)
	if err != nil {
		return err
	}
	m.View = v
	b, err := reader.ReadBool()
	if err != nil {
		return err
	}
	m.InQuorum = b
	return nil
}

func clusterGetViewResponseWriter(message any, writer *messages.Writer, codec messages.Codec) error {
	m := message.(*GetViewResponse)
	if err := writeClusterView(writer, m.View); err != nil {
		return err
	}
	writer.WriteBool(m.InQuorum)
	return writer.Err()
}

func clusterLeaveBroadcastRoundReader(message any, reader *messages.Reader, codec messages.Codec) error {
	m := message.(*LeaveBroadcastRound)
	v, err := reader.ReadInt32()
	if err != nil {
		return err
	}
	m.Round = int(v)
	return nil
}

func clusterLeaveBroadcastRoundWriter(message any, writer *messages.Writer, codec messages.Codec) error {
	m := message.(*LeaveBroadcastRound)
	writer.WriteInt32(int32(m.Round))
	return writer.Err()
}

func clusterJoinRetryTickReader(message any, reader *messages.Reader, codec messages.Codec) error {
	m := message.(*JoinRetryTick)
	nano, err := reader.ReadInt64()
	if err != nil {
		return err
	}
	m.NextDelay = time.Duration(nano)
	return nil
}

func clusterJoinRetryTickWriter(message any, writer *messages.Writer, codec messages.Codec) error {
	m := message.(*JoinRetryTick)
	writer.WriteInt64(int64(m.NextDelay))
	return writer.Err()
}

func clusterForceMemberDownReader(message any, reader *messages.Reader, codec messages.Codec) error {
	m := message.(*ForceMemberDown)
	id, err := reader.ReadString()
	if err != nil {
		return err
	}
	tok, err := reader.ReadString()
	if err != nil {
		return err
	}
	m.NodeID = id
	m.AdminToken = tok
	return nil
}

func clusterForceMemberDownWriter(message any, writer *messages.Writer, codec messages.Codec) error {
	m := message.(*ForceMemberDown)
	writer.WriteString(m.NodeID).WriteString(m.AdminToken)
	return writer.Err()
}

func clusterTriggerViewBroadcastReader(message any, reader *messages.Reader, codec messages.Codec) error {
	m := message.(*TriggerViewBroadcast)
	tok, err := reader.ReadString()
	if err != nil {
		return err
	}
	m.AdminToken = tok
	return nil
}

func clusterTriggerViewBroadcastWriter(message any, writer *messages.Writer, codec messages.Codec) error {
	m := message.(*TriggerViewBroadcast)
	writer.WriteString(m.AdminToken)
	return writer.Err()
}
