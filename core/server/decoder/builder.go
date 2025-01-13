package decoder

import (
	"encoding/gob"
	"github.com/kercylan98/vivid/core"
	"io"
)

var _builder core.DecoderBuilder = &builder{}

func Builder() core.DecoderBuilder {
	return _builder
}

type builder struct{}

func (b *builder) Build(reader io.Reader, provider core.EnvelopeProvider) core.Decoder {
	return &decoder{
		decoder:  gob.NewDecoder(reader),
		provider: provider,
	}
}
