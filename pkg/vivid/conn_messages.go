package vivid

import "github.com/kercylan98/vivid/src/resource"

type ConnHandshakeMessageBuilder interface {
	Build(addr resource.Addr) ConnHandshakeMessage
}

type ConnHandshakeMessage interface {
	GetAddr() resource.Addr
}
