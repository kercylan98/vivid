package core

import (
	"io"
)

type DecoderBuilder interface {
	Build(reader io.Reader, provider EnvelopeProvider) Decoder
}

type Decoder interface {
	Decode() (Envelope, error)
}
