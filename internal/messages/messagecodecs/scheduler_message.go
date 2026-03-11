package messagecodecs

import "github.com/kercylan98/vivid/internal/serialization"

func SchedulerMessageEncoder() serialization.MessageEncoder {
	return serialization.MessageEncoderFN(func(writer *serialization.Writer, message any) error {
		return writer.Write(message).Err()
	})
}

func SchedulerMessageDecoder() serialization.MessageDecoder {
	return serialization.MessageDecoderFN(func(reader *serialization.Reader, message any) error {
		return reader.Read(message)
	})
}
