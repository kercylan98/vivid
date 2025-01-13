package decoder

import (
	"encoding/gob"
	"github.com/kercylan98/vivid/core"
)

var _ core.Decoder = (*decoder)(nil)

type decoder struct {
	decoder  *gob.Decoder
	provider core.EnvelopeProvider
}

func (e *decoder) Decode() (core.Envelope, error) {
	var envelope = e.provider.Provide()
	return envelope, e.decoder.Decode(envelope)
}
