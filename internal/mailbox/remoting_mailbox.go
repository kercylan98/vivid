package mailbox

import (
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/remoting"
	"github.com/kercylan98/vivid/internal/remoting/serialize"
)

var (
	_ vivid.Mailbox = &RemotingMailbox{}
)

func NewRemotingMailbox(advertiseAddress string, poolManager *remoting.ConnectionPoolManager) *RemotingMailbox {
	return &RemotingMailbox{
		advertiseAddress: advertiseAddress,
		poolManager:      poolManager,
	}
}

type RemotingMailbox struct {
	advertiseAddress string
	poolManager      *remoting.ConnectionPoolManager
}

func (m *RemotingMailbox) Enqueue(envelop vivid.Envelop) {
	// 获取或创建连接池
	pool := m.poolManager.GetOrCreatePool(m.advertiseAddress)

	// 使用Sender作为key进行一致性哈希路由
	senderKey := ""
	if sender := envelop.Sender(); sender != nil {
		senderKey = sender.GetAddress() + "@" + sender.GetPath()
	} else {
		// 如果没有Sender，使用目标地址
		senderKey = m.advertiseAddress
	}

	// 获取连接
	conn, err := pool.GetConnection(senderKey)
	if err != nil {
		// TODO: 连接失败时应该重试或使用错误处理机制
		return
	}

	// 序列化 Envelop
	data, err := serialize.SerializeEnvelopWithRemoting(envelop)
	if err != nil {
		// TODO: 序列化失败时应该有错误处理逻辑
		return
	}

	// 获取传输层并发送数据
	transport := remoting.GetTransport()
	if err := transport.Send(conn, data); err != nil {
		// TODO: 发送失败时应该有错误处理逻辑
	}
}
