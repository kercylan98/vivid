package encoder

import (
	"bytes"
	"encoding/gob"
	"github.com/kercylan98/vivid/pkg/vivid"
	"net"
)

var _builder vivid.EncoderBuilder = &builder{}

func Builder() vivid.EncoderBuilder {
	return _builder
}

type builder struct {
	conn     net.Conn
	buffer   *bytes.Buffer
	provider vivid.EnvelopeProvider
}

func (b *builder) Build(buffer *bytes.Buffer) vivid.Encoder {
	return &encoder{
		encoder: gob.NewEncoder(buffer),
	}
}
