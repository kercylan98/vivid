package core

import (
	"bytes"
)

type EncoderBuilder interface {
	Build(buffer *bytes.Buffer) Encoder
}

type Encoder interface {
	Encode(envelope Envelope) error
}
