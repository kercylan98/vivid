package vivid

type Codec interface {
	Encode(message Message) ([]byte, error)
	Decode(message []byte) (Message, error)
}
