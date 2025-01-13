package decoder

import (
	"encoding/gob"
	"github.com/kercylan98/vivid/pkg/vivid"
)

var _ vivid.Decoder = (*decoder)(nil)

type decoder struct {
	decoder  *gob.Decoder
	provider vivid.EnvelopeProvider
}

func (e *decoder) Decode() (vivid.Envelope, error) {
	var envelope = e.provider.Provide()
	return envelope, e.decoder.Decode(envelope)
}
