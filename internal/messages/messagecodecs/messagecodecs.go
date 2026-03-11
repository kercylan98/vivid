package messagecodecs

import "github.com/kercylan98/vivid/internal/serialization"

var (
	genericEncoder = serialization.MessageEncoderFN(func(writer *serialization.Writer, message any) error {
		return writer.Write(message).Err()
	})
	genericDecoder = serialization.MessageDecoderFN(func(reader *serialization.Reader, message any) error {
		return reader.Read(message)
	})
)

// GenericEncoder 通用编码器
func GenericEncoder() serialization.MessageEncoder {
	return genericEncoder
}

// GenericDecoder 通用解码器
func GenericDecoder() serialization.MessageDecoder {
	return genericDecoder
}
