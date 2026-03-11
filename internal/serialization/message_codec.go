package serialization

type MessageCodec interface {
	MessageEncoder
	MessageDecoder
}

type MessageEncoder interface {
	Encode(writer *Writer, message any) error
}

type MessageEncoderFN func(writer *Writer, message any) error

func (fn MessageEncoderFN) Encode(writer *Writer, message any) error {
	return fn(writer, message)
}

type MessageDecoder interface {
	Decode(reader *Reader, message any) error
}

type MessageDecoderFN func(reader *Reader, message any) error

func (fn MessageDecoderFN) Decode(reader *Reader, message any) error {
	return fn(reader, message)
}
