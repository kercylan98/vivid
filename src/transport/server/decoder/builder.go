package decoder

import (
	"encoding/gob"
	"github.com/kercylan98/vivid/pkg/vivid"
	"io"
)

var _builder vivid.DecoderBuilder = &builder{}

func Builder() vivid.DecoderBuilder {
	return _builder
}

type builder struct{}

func (b *builder) Build(reader io.Reader, provider vivid.EnvelopeProvider) vivid.Decoder {
	return &decoder{
		decoder:  gob.NewDecoder(reader),
		provider: provider,
	}
}
