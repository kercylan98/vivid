package serialize

import (
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/messages"
)

// EncodeEnvelopWithRemoting Envelope 线格式（与 Decode 顺序严格一致）：
//
//	[4B payloadLen][payload]
//	[4B nameLen][messageName] [1B system]
//	[agentAddr 串] [agentPath 串] [senderAddr 串] [senderPath 串] [receiverAddr 串] [receiverPath 串]
//
// 每串为 4 字节长度 + UTF-8；Ref 为 nil 时写空串。
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
	if agent := envelop.Agent(); agent != nil {
		agentAddr, agentPath = agent.GetAddress(), agent.GetPath()
	}
	var senderAddr, senderPath string
	if s := envelop.Sender(); s != nil {
		senderAddr, senderPath = s.GetAddress(), s.GetPath()
	}
	var receiverAddr, receiverPath string
	if r := envelop.Receiver(); r != nil {
		receiverAddr, receiverPath = r.GetAddress(), r.GetPath()
	}

	if err = writer.WriteFrom(
		messageDesc.MessageName(),
		envelop.System(),
		agentAddr, agentPath,
		senderAddr, senderPath,
		receiverAddr, receiverPath,
	); err != nil {
		return nil, err
	}
	// 必须拷贝：返回后 defer 会 ReleaseWriterToPool 并 Reset，data 会变空，调用方 append(lengthBuf, data...) 会拿到空内容导致对端解码错位（Ref 乱码/缺字段）
	out := make([]byte, len(writer.Bytes()))
	copy(out, writer.Bytes())
	return out, nil
}

// DecodeEnvelopWithRemoting 解码的字段顺序必须与 EncodeEnvelopWithRemoting 线格式一致：
// messageData, messageName, system, agentAddr, agentPath, senderAddr, senderPath, receiverAddr, receiverPath。
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

	var messageData []byte
	var messageName string
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
