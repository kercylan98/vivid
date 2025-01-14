package encoder

import (
	"encoding/gob"
	vivid2 "github.com/kercylan98/vivid/.discord/pkg/vivid"
)

var _ vivid2.Encoder = (*encoder)(nil)

type encoder struct {
	encoder *gob.Encoder
}

func (e *encoder) Encode(envelope vivid2.Envelope) error {
	return e.encoder.Encode(envelope)
}
