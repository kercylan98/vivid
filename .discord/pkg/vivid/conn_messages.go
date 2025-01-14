package vivid

import (
	"github.com/kercylan98/vivid/.discord/src/resource"
)

type ConnHandshakeMessageBuilder interface {
	Build(addr resource.Addr) ConnHandshakeMessage
}

type ConnHandshakeMessage interface {
	GetAddr() resource.Addr
}
