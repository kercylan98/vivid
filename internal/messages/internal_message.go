package messages

type InternalMessage interface {
	MessageType() uint32
	Read(reader *Reader) error
	Write(writer *Writer) error
}
