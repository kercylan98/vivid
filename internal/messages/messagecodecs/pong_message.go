package messagecodecs

import "github.com/kercylan98/vivid/internal/serialization"

func PongMessageEncoder() serialization.MessageEncoder {
	return serialization.MessageEncoderFN(func(writer *serialization.Writer, message any) error {
		return writer.Write(message).Err()
	})
}

func PongMessageDecoder() serialization.MessageDecoder {
	return serialization.MessageDecoderFN(func(reader *serialization.Reader, message any) error {
		return reader.Read(message)
	})
}
