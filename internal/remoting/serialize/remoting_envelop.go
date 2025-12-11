package serialize

import (
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/messages"
)

func SerializeEnvelopWithRemoting(envelop vivid.Envelop) (data []byte, err error) {
	var writer = messages.NewWriterFromPool()
	defer messages.ReleaseWriterToPool(writer)
	messageDesc := messages.QueryMessageDesc(envelop.Message())
	err = messages.SerializeRemotingMessage(writer, messageDesc, envelop.Message())
	if err != nil {
		return nil, err
	}

	// | data | messageName | system | agent | sender |
	writer.WriteFrom(
		messageDesc.MessageName(), // 消息名称
		envelop.System(),          // 是否为系统消息
		envelop.Agent(),           // 被代理的 ActorRef
		envelop.Sender(),          // 消息的发送者 ActorRef
	)

	return writer.Bytes(), nil

}

func DeserializeEnvelopWithRemoting(data []byte, provider vivid.EnvelopProvider) (envelop vivid.Envelop, err error) {
	reader := messages.NewReaderFromPool(data)
	defer messages.ReleaseReaderToPool(reader)

	var (
		messageName     string
		system          bool
		agent           vivid.ActorRef
		sender          vivid.ActorRef
		messageData     []byte
		messageInstance any
	)

	// | data | messageName | system | agent | sender |
	if err = reader.ReadInto(&messageData, &messageName, &system, &agent, &sender); err != nil {
		return nil, err
	}

	if messageDesc := messages.QueryMessageDescByName(messageName); !messageDesc.IsOutside() {
		reader.Reset(messageData)
		messageInstance, err = messages.DeserializeRemotingMessage(reader, messageDesc)
		if err != nil {
			return nil, err
		}
	}

	return provider.Provide(system, agent, sender, messageInstance), nil
}
