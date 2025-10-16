package registry

import (
	"io"

	"github.com/kercylan98/vivid/pkg/runtime"
	"github.com/kercylan98/vivid/pkg/serializer"
)

var _ runtime.Process = (*broker)(nil)

func newBroker(processId *runtime.ProcessID, serializer serializer.NameSerializer, writer io.Writer) *broker {
	return &broker{
		receiver:   processId,
		serializer: serializer,
		writer:     writer,
	}
}

type broker struct {
	receiver   *runtime.ProcessID        // 接收者
	serializer serializer.NameSerializer // 序列化器
	writer     io.Writer                 // 写入器
}

func (b *broker) OnMessage(sender *runtime.ProcessID, messageType runtime.MessageType, message runtime.Message) error {
	// 构建远程消息
	messageName, messageBytes, err := b.serializer.Serialize(message)
	if err != nil {
		return err
	}

	// 序列化消息
	remoteMessage := newRemoteMessageWithPool(0, b.receiver, messageType, messageName, messageBytes)
	remoteMessageBytes, err := remoteMessage.encode()
	if err != nil {
		return err
	}

	// 发送远程消息
	if _, err := b.writer.Write(remoteMessageBytes); err != nil {
		return err
	}
	return nil
}
