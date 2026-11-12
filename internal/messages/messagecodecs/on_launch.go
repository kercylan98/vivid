package messagecodecs

import "github.com/kercylan98/vivid/internal/serialization"

func OnLaunchEncoder() serialization.MessageEncoder {
	return serialization.MessageEncoderFN(func(writer *serialization.Writer, message any) error {
		return nil
	})
}

func OnLaunchDecoder() serialization.MessageDecoder {
	return serialization.MessageDecoderFN(func(reader *serialization.Reader, message any) error {
		return nil
	})
}
