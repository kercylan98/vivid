package remoting

import (
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/serialization"
)

func ensureEnvelopReceiverAddress(envelop vivid.Envelop) string {
	if r := envelop.Receiver(); r != nil {
		return r.GetAddress()
	}
	return ""
}

func encodeEnvelop(codec *serialization.VividCodec, envelop vivid.Envelop) ([]byte, error) {
	var writer = serialization.GetWriter(codec)
	defer serialization.PutWriter(writer)

	var sender, receiver string
	if s := envelop.Sender(); s != nil {
		sender = s.String()
	}
	if r := envelop.Receiver(); r != nil {
		receiver = r.String()
	}

	writer.Write(
		envelop.System(),
		sender,
		receiver,
	)

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

	if err = reader.Read(&system, &sender, &receiver); err != nil {
		return
	}

	message, err = codec.Decode(reader)
	return
}
