package encoder

import (
	"encoding/gob"
	"github.com/kercylan98/vivid/core"
)

var _ core.Encoder = (*encoder)(nil)

type encoder struct {
	encoder *gob.Encoder
}

func (e *encoder) Encode(envelope core.Envelope) error {
	return e.encoder.Encode(envelope)
}
