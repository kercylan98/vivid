package serialization

type Codec interface {
	Encode(writer *Writer, message any) error
	Decode(reader *Reader) (any, error)
}
