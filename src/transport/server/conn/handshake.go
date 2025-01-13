package conn

import (
	"github.com/kercylan98/vivid/pkg/vivid"
	"github.com/kercylan98/vivid/src/resource"
)

func init() {
	vivid.GetMessageRegister().RegisterName("vivid.transport.server.conn.handshake", new(handshake))
}

var _ vivid.ConnHandshakeMessage = (*handshake)(nil)

type handshake struct {
	Addr resource.Addr
}

func (h *handshake) GetAddr() resource.Addr {
	return h.Addr
}
