// 集群消息在 remoting 层使用内部序列化，通过 internal/messages 注册并实现 reader/writer。
// NodeState、ClusterView 使用二进制格式读写，不依赖应用 Codec，避免 gob 解码 interface{} 失败。

package cluster

import (
	"fmt"
	"sort"
	"time"

	"github.com/kercylan98/vivid/internal/messages"
)

func init() {
	messages.RegisterInternalMessage[*JoinRequest](
		"clusterJoinRequest", clusterJoinRequestReader, clusterJoinRequestWriter)
	messages.RegisterInternalMessage[*JoinResponse](
		"clusterJoinResponse", clusterJoinResponseReader, clusterJoinResponseWriter)
	messages.RegisterInternalMessage[*GossipMessage](
		"clusterGossip", clusterGossipReader, clusterGossipWriter)
	messages.RegisterInternalMessage[*GossipTick](
		"clusterGossipTick", clusterNoopReader, clusterNoopWriter)
	messages.RegisterInternalMessage[*GossipCrossDCTick](
		"clusterGossipCrossDCTick", clusterNoopReader, clusterNoopWriter)
	messages.RegisterInternalMessage[*FailureDetectionTick](
		"clusterFailureDetectionTick", clusterNoopReader, clusterNoopWriter)
	messages.RegisterInternalMessage[*GetViewRequest](
		"clusterGetViewRequest", clusterNoopReader, clusterNoopWriter)
	messages.RegisterInternalMessage[*GetViewResponse](
		"clusterGetViewResponse", clusterGetViewResponseReader, clusterGetViewResponseWriter)
	messages.RegisterInternalMessage[*LeaveRequest](
		"clusterLeaveRequest", clusterNoopReader, clusterNoopWriter)
	messages.RegisterInternalMessage[*LeaveAck](
		"clusterLeaveAck", clusterNoopReader, clusterNoopWriter)
	messages.RegisterInternalMessage[*ExitingReady](
		"clusterExitingReady", clusterNoopReader, clusterNoopWriter)
	messages.RegisterInternalMessage[*LeaveBroadcastRound](
		"clusterLeaveBroadcastRound", clusterLeaveBroadcastRoundReader, clusterLeaveBroadcastRoundWriter)
	messages.RegisterInternalMessage[*JoinRetryTick](
		"clusterJoinRetryTick", clusterJoinRetryTickReader, clusterJoinRetryTickWriter)
	messages.RegisterInternalMessage[*ForceMemberDown](
		"clusterForceMemberDown", clusterForceMemberDownReader, clusterForceMemberDownWriter)
	messages.RegisterInternalMessage[*TriggerViewBroadcast](
		"clusterTriggerViewBroadcast", clusterTriggerViewBroadcastReader, clusterTriggerViewBroadcastWriter)
}

func clusterNoopReader(message any, reader *messages.Reader, codec messages.Codec) error { return nil }
func clusterNoopWriter(message any, writer *messages.Writer, codec messages.Codec) error { return nil }

// writeNodeState 将 NodeState 按二进制格式写入，nil 写长度 0。
func writeNodeState(w *messages.Writer, n *NodeState) error {
	if n == nil {
		w.WriteUint32(0)
		return w.Err()
	}
	w.WriteUint32(1)
	return writerWriteNodeStateBody(w, n)
}

func writerWriteNodeStateBody(w *messages.Writer, n *NodeState) error {
	if err := w.WriteFrom(n.ID, n.ClusterName, n.Address, int32(n.Generation), n.Timestamp, n.SeqNo, int32(n.Status), n.Unreachable, n.LastSeen, n.LogicalClock); err != nil {
		return err
	}
	if err := writeMapStringString(w, n.Metadata); err != nil {
		return err
	}
	if err := writeMapStringString(w, n.Labels); err != nil {
		return err
	}
	return w.WriteFrom(n.Checksum)
}

// maxMapEntries 反序列化时单 map 最大条目数，防止损坏数据导致异常分配或死循环
const maxMapEntries = 65536

// writeMapStringString 按 key 排序写入，保证同一 map 序列化结果一致（便于校验和与比对）。
// 任一步写入失败会立即返回，避免写出“长度正确但内容不完整”的 map 导致解码错位。
func writeMapStringString(w *messages.Writer, m map[string]string) error {
	if m == nil {
		w.WriteUint32(0)
		return w.Err()
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	w.WriteUint32(uint32(len(keys)))
	if w.Err() != nil {
		return w.Err()
	}
	for _, k := range keys {
		w.WriteString(k).WriteString(m[k])
		if w.Err() != nil {
			return w.Err()
		}
	}
	return nil
}

func readNodeState(r *messages.Reader) (*NodeState, error) {
	var n uint32
	if err := r.ReadInto(&n); err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, nil
	}
	return readerReadNodeStateBody(r)
}

func readerReadNodeStateBody(r *messages.Reader) (*NodeState, error) {
	var id, clusterName, address string
	var gen int32
	var timestamp, lastSeen int64
	var seqNo, logicalClock uint64
	var status int32
	var unreachable bool
	var checksum uint32
	if err := r.ReadInto(&id, &clusterName, &address, &gen, &timestamp, &seqNo, &status, &unreachable, &lastSeen, &logicalClock); err != nil {
		return nil, err
	}
	metadata, err := readMapStringString(r)
	if err != nil {
		return nil, err
	}
	labels, err := readMapStringString(r)
	if err != nil {
		return nil, err
	}
	if err := r.ReadInto(&checksum); err != nil {
		return nil, err
	}
	return &NodeState{
		ID: id, ClusterName: clusterName, Address: address,
		Generation: int(gen), Timestamp: timestamp, SeqNo: seqNo,
		Status: MemberStatus(status), Unreachable: unreachable, LastSeen: lastSeen, LogicalClock: logicalClock,
		Metadata: metadata, Labels: labels, Checksum: checksum,
	}, nil
}

func readMapStringString(r *messages.Reader) (map[string]string, error) {
	n, err := r.ReadUint32()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, nil
	}
	if n > maxMapEntries {
		return nil, fmt.Errorf("map length %d exceeds max %d", n, maxMapEntries)
	}
	m := make(map[string]string, n)
	for i := uint32(0); i < n; i++ {
		k, err := r.ReadString()
		if err != nil {
			return nil, err
		}
		v, err := r.ReadString()
		if err != nil {
			return nil, err
		}
		m[k] = v
	}
	return m, nil
}

// writeClusterView 将 ClusterView 按二进制格式写入，nil 写长度 0。
func writeClusterView(w *messages.Writer, v *ClusterView) error {
	if v == nil {
		w.WriteUint32(0)
		return w.Err()
	}
	w.WriteUint32(1)
	if err := w.WriteFrom(v.ViewID, v.Epoch, v.Timestamp); err != nil {
		return err
	}
	memberIDs := make([]string, 0, len(v.Members))
	for id := range v.Members {
		memberIDs = append(memberIDs, id)
	}
	sort.Strings(memberIDs)
	w.WriteUint32(uint32(len(memberIDs)))
	for _, id := range memberIDs {
		state := v.Members[id]
		w.WriteString(id)
		if state != nil {
			w.WriteUint8(1)
			if err := writerWriteNodeStateBody(w, state); err != nil {
				return err
			}
		} else {
			w.WriteUint8(0)
		}
	}
	if err := w.WriteFrom(int32(v.HealthyCount), int32(v.UnhealthyCount), int32(v.QuorumSize)); err != nil {
		return err
	}
	if err := WriteVersionVector(w, v.VersionVector); err != nil {
		return err
	}
	return w.WriteFrom(v.ProtocolVersion, int32(v.MaxVersionVectorEntries))
}

func readClusterView(r *messages.Reader) (*ClusterView, error) {
	var n uint32
	if err := r.ReadInto(&n); err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, nil
	}
	var viewID string
	var epoch, timestamp int64
	var memLen uint32
	if err := r.ReadInto(&viewID, &epoch, &timestamp, &memLen); err != nil {
		return nil, err
	}
	members := make(map[string]*NodeState, memLen)
	for i := uint32(0); i < memLen; i++ {
		var id string
		var has uint8
		if err := r.ReadInto(&id, &has); err != nil {
			return nil, err
		}
		if has != 0 {
			state, err := readerReadNodeStateBody(r)
			if err != nil {
				return nil, err
			}
			members[id] = state
		}
	}
	var healthy, unhealthy, quorum int32
	if err := r.ReadInto(&healthy, &unhealthy, &quorum); err != nil {
		return nil, err
	}
	vv, err := ReadVersionVector(r)
	if err != nil {
		return nil, err
	}
	var protoVer uint16
	var maxVV int32
	if err := r.ReadInto(&protoVer, &maxVV); err != nil {
		return nil, err
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
	return reader.ReadInto(&m.AuthToken)
}

func clusterJoinRequestWriter(message any, writer *messages.Writer, codec messages.Codec) error {
	m := message.(*JoinRequest)
	if err := writeNodeState(writer, m.NodeState); err != nil {
		return err
	}
	return writer.WriteFrom(m.AuthToken)
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
	return reader.ReadInto(&m.InQuorum)
}

func clusterGetViewResponseWriter(message any, writer *messages.Writer, codec messages.Codec) error {
	m := message.(*GetViewResponse)
	if err := writeClusterView(writer, m.View); err != nil {
		return err
	}
	return writer.WriteFrom(m.InQuorum)
}

func clusterLeaveBroadcastRoundReader(message any, reader *messages.Reader, codec messages.Codec) error {
	m := message.(*LeaveBroadcastRound)
	var round int32
	if err := reader.ReadInto(&round); err != nil {
		return err
	}
	m.Round = int(round)
	return nil
}

func clusterLeaveBroadcastRoundWriter(message any, writer *messages.Writer, codec messages.Codec) error {
	m := message.(*LeaveBroadcastRound)
	return writer.WriteFrom(int32(m.Round))
}

func clusterJoinRetryTickReader(message any, reader *messages.Reader, codec messages.Codec) error {
	m := message.(*JoinRetryTick)
	var nano int64
	if err := reader.ReadInto(&nano); err != nil {
		return err
	}
	m.NextDelay = time.Duration(nano)
	return nil
}

func clusterJoinRetryTickWriter(message any, writer *messages.Writer, codec messages.Codec) error {
	m := message.(*JoinRetryTick)
	return writer.WriteFrom(int64(m.NextDelay))
}

func clusterForceMemberDownReader(message any, reader *messages.Reader, codec messages.Codec) error {
	m := message.(*ForceMemberDown)
	return reader.ReadInto(&m.NodeID, &m.AdminToken)
}

func clusterForceMemberDownWriter(message any, writer *messages.Writer, codec messages.Codec) error {
	m := message.(*ForceMemberDown)
	return writer.WriteFrom(m.NodeID, m.AdminToken)
}

func clusterTriggerViewBroadcastReader(message any, reader *messages.Reader, codec messages.Codec) error {
	m := message.(*TriggerViewBroadcast)
	return reader.ReadInto(&m.AdminToken)
}

func clusterTriggerViewBroadcastWriter(message any, writer *messages.Writer, codec messages.Codec) error {
	m := message.(*TriggerViewBroadcast)
	return writer.WriteFrom(m.AdminToken)
}
