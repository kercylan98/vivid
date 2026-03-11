package remoting

import (
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/serialization"
)

func encodeEnvelop(codec *serialization.VividCodec, envelop vivid.Envelop) ([]byte, error) {
	var writer = serialization.GetWriter(codec)
	defer serialization.PutWriter(writer)

	// 数据准备
	var sender, receiver string
	if s := envelop.Sender(); s != nil {
		sender = s.String()
	}
	if r := envelop.Receiver(); r != nil {
		receiver = r.String()
	}

	// 写入基本信息
	writer.Write(
		envelop.System(),
		sender,
		receiver,
	)

	// 写入消息数据
	if err := codec.Encode(writer, envelop.Message()); err != nil {
		return nil, err
	}

	return writer.Bytes(), nil
}

func decodeEnvelop(codec *serialization.VividCodec, data []byte) (
	system bool, sender, receiver string, message any,
	err error,
) {
	var reader = serialization.GetReader(codec, data)
	defer serialization.PutReader(reader)

	// 读取基本信息
	if err = reader.Read(&system, &sender, &receiver); err != nil {
		return
	}

	// 读取消息数据
	message, err = codec.Decode(reader)
	return
}
