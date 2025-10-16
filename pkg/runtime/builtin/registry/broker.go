package registry

import (
	"github.com/kercylan98/vivid/pkg/runtime"
	"github.com/kercylan98/vivid/pkg/serializer"
)

var _ runtime.Process = (*broker)(nil)

// newBroker 创建一个远程进程代理。
func newBroker(receiver *runtime.ProcessID, serializer serializer.NameSerializer, pool *connectionPool) *broker {
	return &broker{
		receiver:   receiver,
		serializer: serializer,
		pool:       pool,
	}
}

// broker 实现了 runtime.Process 接口，作为远程进程的本地代理。
type broker struct {
	receiver   *runtime.ProcessID        // 接收者
	serializer serializer.NameSerializer // 序列化器
	pool       *connectionPool           // 连接池
}

// OnMessage 实现 runtime.Process 接口。
func (b *broker) OnMessage(sender *runtime.ProcessID, messageType runtime.MessageType, message runtime.Message) error {
	// 序列化消息
	messageName, messageBytes, err := b.serializer.Serialize(message)
	if err != nil {
		return err
	}

	// 构建数据包
	var senderPath string
	if sender != nil {
		senderPath = sender.Path()
	}
	receiverPath := b.receiver.Path()
	isSystem := messageType == runtime.MessageTypeSystem

	packet := NewPacket(0, senderPath, receiverPath, messageName, messageBytes, isSystem)

	// 通过连接池发送
	if err := b.pool.Send(b.receiver.Address(), packet); err != nil {
		ReleasePacket(packet)
		return err
	}

	// 注意：packet 会在连接池的发送循环中释放
	return nil
}
