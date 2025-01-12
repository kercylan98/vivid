package encoder

import (
	"bytes"
	"encoding/gob"
	"github.com/kercylan98/vivid/core"
	"net"
)

var _builder core.EncoderBuilder = &builder{}

func Builder() core.EncoderBuilder {
	return _builder
}

type builder struct {
	conn     net.Conn
	buffer   *bytes.Buffer
	provider core.EnvelopeProvider
}

func (b *builder) Build(buffer *bytes.Buffer) core.Encoder {
	return &encoder{
		encoder: gob.NewEncoder(buffer),
	}
}
