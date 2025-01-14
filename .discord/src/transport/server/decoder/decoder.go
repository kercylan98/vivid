package decoder

import (
	"encoding/gob"
	vivid2 "github.com/kercylan98/vivid/.discord/pkg/vivid"
)

var _ vivid2.Decoder = (*decoder)(nil)

type decoder struct {
	decoder  *gob.Decoder
	provider vivid2.EnvelopeProvider
}

func (e *decoder) Decode() (vivid2.Envelope, error) {
	var envelope = e.provider.Provide()
	return envelope, e.decoder.Decode(envelope)
}
