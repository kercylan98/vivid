package serialize

import (
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/messages"
)

func EncodeEnvelopWithRemoting(codec vivid.Codec, envelop vivid.Envelop) (data []byte, err error) {
	var messageDesc = messages.QueryMessageDesc(envelop.Message())
	var writer = messages.NewWriterFromPool()
	defer messages.ReleaseWriterToPool(writer)
	if messageDesc.IsOutside() {
		// 外部消息序列化
		data, err = codec.Encode(envelop.Message())
		if err != nil {
			return nil, err
		}
		writer.WriteBytesWithLength(data, 4)
	} else {
		// 内部消息序列化
		err = messages.SerializeRemotingMessage(codec, writer, messageDesc, envelop.Message())
		if err != nil {
			return nil, err
		}
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

	// 返回副本，避免调用方持有 pooled writer 的 buf 引用导致被复用覆盖
	// （在高并发/多连接同时建连时，曾出现 decode failed: read index 1 failed: unexpected EOF）
	b := writer.Bytes()
	out := make([]byte, len(b))
	copy(out, b)
	return out, nil
}

func DecodeEnvelopWithRemoting(codec vivid.Codec, data []byte) (
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

	// | data | messageName | system | agent | sender | receiver |
	if err = reader.ReadInto(&messageData, &messageName, &system, &agentAddr, &agentPath, &senderAddr, &senderPath, &receiverAddr, &receiverPath); err != nil {
		return
	}

	if messageDesc := messages.QueryMessageDescByName(messageName); !messageDesc.IsOutside() {
		// 内部消息反序列化
		reader.Reset(messageData)
		messageInstance, err = messages.DeserializeRemotingMessage(codec, reader, messageDesc)
		if err != nil {
			return
		}
	} else {
		// 外部消息反序列化
		messageInstance, err = codec.Decode(messageData)
		if err != nil {
			return
		}
	}

	return
}
