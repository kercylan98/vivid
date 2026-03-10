package messages

type Serializable interface {
	Serialize(writer *Writer) error
	Deserialize(reader *Reader) error
}
