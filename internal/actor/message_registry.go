package actor

import (
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/serialization"
)

func newMessageRegistry(codec *serialization.VividCodec) *messageRegistry {
	return &messageRegistry{
		codec: codec,
	}
}

type messageRegistry struct {
	class string
	codec *serialization.VividCodec
	err   error
}

func (r *messageRegistry) SetClass(class string) *messageRegistry {
	r.class = class
	return r
}

func (r *messageRegistry) RegisterMessage(name string, message vivid.MessageCodec) {
	if r.err != nil {
		return
	}
	r.err = r.codec.RegisterMessage(r.class, name, message)
}

func (r *messageRegistry) RegisterMessageWithEncoderAndDecoder(name string, message any, encoder vivid.MessageEncoder, decoder vivid.MessageDecoder) {
	if r.err != nil {
		return
	}
	r.err = r.codec.RegisterMessageWithEncoderAndDecoder(r.class, name, message, encoder, decoder)
}

func (r *messageRegistry) Err() error {
	return r.err
}
