package decoder

import (
	"encoding/gob"
	vivid2 "github.com/kercylan98/vivid/.discord/pkg/vivid"
	"io"
)

var _builder vivid2.DecoderBuilder = &builder{}

func Builder() vivid2.DecoderBuilder {
	return _builder
}

type builder struct{}

func (b *builder) Build(reader io.Reader, provider vivid2.EnvelopeProvider) vivid2.Decoder {
	return &decoder{
		decoder:  gob.NewDecoder(reader),
		provider: provider,
	}
}
