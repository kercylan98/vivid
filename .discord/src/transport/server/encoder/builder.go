package encoder

import (
	"bytes"
	"encoding/gob"
	vivid2 "github.com/kercylan98/vivid/.discord/pkg/vivid"
	"net"
)

var _builder vivid2.EncoderBuilder = &builder{}

func Builder() vivid2.EncoderBuilder {
	return _builder
}

type builder struct {
	conn     net.Conn
	buffer   *bytes.Buffer
	provider vivid2.EnvelopeProvider
}

func (b *builder) Build(buffer *bytes.Buffer) vivid2.Encoder {
	return &encoder{
		encoder: gob.NewEncoder(buffer),
	}
}
