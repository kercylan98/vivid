package serialize

import (
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/messages"
)

func EncodeEnvelopWithRemoting(envelop vivid.Envelop) (data []byte, err error) {
	var writer = messages.NewWriterFromPool()
	defer messages.ReleaseWriterToPool(writer)
	messageDesc := messages.QueryMessageDesc(envelop.Message())
	err = messages.SerializeRemotingMessage(writer, messageDesc, envelop.Message())
	if err != nil {
		return nil, err
	}

	var agentAddr, agentPath string
	var senderAddr, senderPath = envelop.Sender().GetAddress(), envelop.Sender().GetPath()
	var receiverAddr, receiverPath = envelop.Receiver().GetAddress(), envelop.Receiver().GetPath()
	if agent := envelop.Agent(); agent != nil {
		agentAddr, agentPath = agent.GetAddress(), agent.GetPath()
	}

	if err = writer.WriteFrom(
		messageDesc.MessageName(), // 消息名称
		envelop.System(),          // 是否为系统消息
		agentAddr, agentPath,      // 被代理的 ActorRef
		senderAddr, senderPath, // 消息的发送者 ActorRef
		receiverAddr, receiverPath, // 消息接收人
	); err != nil {
		return nil, err
	}

	// | data | messageName | system | agent | sender |
	return writer.Bytes(), nil
}

func DecodeEnvelopWithRemoting(data []byte) (
	system bool,
	agentAddr, agentPath string,
	senderAddr, senderPath string,
	receiverAddr, receiverPath string,
	messageInstance any,
	err error,
) {
	reader := messages.NewReaderFromPool(data)
	defer messages.ReleaseReaderToPool(reader)

	var messageName string
	var messageData []byte

	// | data | messageName | system | agent | sender |
	if err = reader.ReadInto(&messageData, &messageName, &system, &agentAddr, &agentPath, &senderAddr, &senderPath, &receiverAddr, &receiverPath); err != nil {
		return
	}

	if messageDesc := messages.QueryMessageDescByName(messageName); !messageDesc.IsOutside() {
		reader.Reset(messageData)
		messageInstance, err = messages.DeserializeRemotingMessage(reader, messageDesc)
		if err != nil {
			return
		}
	}

	return
}
