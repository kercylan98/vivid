package vivid

import (
	"github.com/kercylan98/vivid/.discord/src/resource"
)

type ClientBuilder interface {
	Build(addr resource.Addr) Client
}

type Client interface {
	Read(b []byte) (n int, err error)
	Write(b []byte) (n int, err error)
	Close() error

	GetEnvelopeChannel() <-chan Envelope
}
