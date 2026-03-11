package vivid

import "github.com/kercylan98/vivid/internal/serialization"

type (
	MessageCodec   = serialization.MessageCodec
	MessageEncoder = serialization.MessageEncoder
	MessageDecoder = serialization.MessageDecoder
)

type MessageRegister interface {
	Register(registry MessageRegistry)
}

type MessageRegisterFN func(registry MessageRegistry)

func (fn MessageRegisterFN) Register(registry MessageRegistry) {
	fn(registry)
}

type MessageRegistry interface {
	RegisterMessage(name string, message MessageCodec)
	RegisterMessageWithEncoderAndDecoder(name string, message any, encoder MessageEncoder, decoder MessageDecoder)
}
