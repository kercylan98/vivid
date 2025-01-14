package conn

import (
	"github.com/kercylan98/vivid/.discord/pkg/vivid"
	"github.com/kercylan98/vivid/.discord/src/resource"
)

var (
	_                 vivid.ConnHandshakeMessageBuilder = (*handshakeBuilder)(nil)
	_handshakeBuilder vivid.ConnHandshakeMessageBuilder = &handshakeBuilder{}
)

func HandshakeBuilder() vivid.ConnHandshakeMessageBuilder {
	return _handshakeBuilder
}

type handshakeBuilder struct{}

func (h *handshakeBuilder) Build(addr resource.Addr) vivid.ConnHandshakeMessage {
	return &handshake{
		Addr: addr,
	}
}
