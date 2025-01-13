package encoder

import (
	"encoding/gob"
	"github.com/kercylan98/vivid/pkg/vivid"
)

var _ vivid.Encoder = (*encoder)(nil)

type encoder struct {
	encoder *gob.Encoder
}

func (e *encoder) Encode(envelope vivid.Envelope) error {
	return e.encoder.Encode(envelope)
}
