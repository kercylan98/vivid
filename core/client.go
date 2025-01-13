package core

type ClientBuilder interface {
	Build(addr Addr) Client
}

type Client interface {
	Read(b []byte) (n int, err error)
	Write(b []byte) (n int, err error)
	Close() error

	GetEnvelopeChannel() <-chan Envelope
}
