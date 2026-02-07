package cluster

import (
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/messages"
)

func init() {
	vivid.RegisterCustomMessage[*nodeMessageAsGetNodesRequest]("nodeMessageAsGetNodesRequest", nodeMessageAsGetNodesRequestReader, nodeMessageAsGetNodesRequestWriter)
	vivid.RegisterCustomMessage[*nodeMessageAsGetNodesResponse]("getNodesResponse", nodeMessageAsGetNodesResponseReader, nodeMessageAsGetNodesResponseWriter)
	vivid.RegisterCustomMessage[*nodeMessageAsLeaveCluster]("leaveCluster", nodeMessageAsLeaveClusterReader, nodeMessageAsLeaveClusterWriter)
}

// nodeMessageAsGetNodesRequest 表示向远程 NodeActor 请求其当前成员列表的协议消息。
// ClusterName 用于仅与同集群节点交换；空串表示不校验集群名。
type nodeMessageAsGetNodesRequest struct {
	ClusterName string
}

func nodeMessageAsGetNodesRequestReader(message any, reader *messages.Reader, codec messages.Codec) error {
	m := message.(*nodeMessageAsGetNodesRequest)
	s, err := reader.ReadString()
	if err != nil {
		return err
	}
	m.ClusterName = s
	return nil
}

func nodeMessageAsGetNodesRequestWriter(message any, writer *messages.Writer, codec messages.Codec) error {
	m := message.(*nodeMessageAsGetNodesRequest)
	writer.WriteString(m.ClusterName)
	return writer.Err()
}

// nodeMessageAsGetNodesResponse 表示对 GetNodesRequest 的回复。
// 请求方仅当 ClusterName 与自身一致时合并 Members，并更新对应成员的 LastSeen。
type nodeMessageAsGetNodesResponse struct {
	ClusterName string
	Members     []vivid.ClusterMemberInfo
}

func nodeMessageAsGetNodesResponseReader(message any, reader *messages.Reader, codec messages.Codec) error {
	m := message.(*nodeMessageAsGetNodesResponse)
	cn, err := reader.ReadString()
	if err != nil {
		return err
	}
	m.ClusterName = cn
	n, err := reader.ReadUint32()
	if err != nil {
		return err
	}
	if n > maxGetNodesResponseMembers {
		n = maxGetNodesResponseMembers
	}
	m.Members = make([]vivid.ClusterMemberInfo, 0, int(n))
	for i := uint32(0); i < n; i++ {
		addr, err := reader.ReadString()
		if err != nil {
			return err
		}
		ver, err := reader.ReadString()
		if err != nil {
			return err
		}
		m.Members = append(m.Members, vivid.ClusterMemberInfo{Address: addr, Version: ver})
	}
	return nil
}

func nodeMessageAsGetNodesResponseWriter(message any, writer *messages.Writer, codec messages.Codec) error {
	m := message.(*nodeMessageAsGetNodesResponse)
	writer.WriteString(m.ClusterName)
	writer.WriteUint32(uint32(len(m.Members)))
	for _, mi := range m.Members {
		writer.WriteString(mi.Address)
		writer.WriteString(mi.Version)
	}
	return writer.Err()
}

// publicMessageAsGetNodesQuery 用于向本节点 NodeActor 查询当前成员列表（本地 Ask），返回 GetNodesResult。
type publicMessageAsGetNodesQuery struct {
	ClusterName string
}

// publicMessageAsGetNodesResult 为 GetNodesQuery 的回复。
type publicMessageAsGetNodesResult struct {
	Members []vivid.ClusterMemberInfo
}

// publicMessageAsMembersUpdated 由外部服务发现向 NodeActor 推送最新节点列表时使用。
type publicMessageAsMembersUpdated struct {
	nodes []string
}

// publicMessageAsSetNodeVersion 用于配置本节点版本号（仅本地投递），会体现在成员信息中。
type publicMessageAsSetNodeVersion struct {
	version string
}

// nodeMessageAsLeaveCluster 表示发送方节点主动离开集群；收到方应将发送方从成员表移除，无需等待故障检测超时。
// 由即将下线的节点向其已知成员广播（跨节点需注册 Remoting）。
type nodeMessageAsLeaveCluster struct{}

func nodeMessageAsLeaveClusterReader(message any, reader *messages.Reader, codec messages.Codec) error {
	return nil
}

func nodeMessageAsLeaveClusterWriter(message any, writer *messages.Writer, codec messages.Codec) error {
	return nil
}

// publicMessageAsInitiateLeave 用于让本节点主动离开集群（仅本地投递）。向 NodeActor Tell 后，其会向当前已知成员发送 LeaveCluster，
// 并将自身从本地成员表移除，其它节点收到 LeaveCluster 后会立即移除本节点，实现及时更新。
type publicMessageAsInitiateLeave struct{}

// publicMessageAsGetClusterState 用于向本节点 NodeActor 查询当前选主与多数派状态（本地 Ask）。
type publicMessageAsGetClusterState struct{}

// publicMessageAsGetClusterStateResult 为 GetClusterState 的回复，供 ClusterContext.Leader/IsLeader/InQuorum 使用。
type publicMessageAsGetClusterStateResult struct {
	LeaderAddress string
	InQuorum      bool
}
