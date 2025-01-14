package conn

import (
	vivid2 "github.com/kercylan98/vivid/.discord/pkg/vivid"
	"github.com/kercylan98/vivid/.discord/src/resource"
)

func init() {
	vivid2.GetMessageRegister().RegisterName("vivid.transport.server.conn.handshake", new(handshake))
}

var _ vivid2.ConnHandshakeMessage = (*handshake)(nil)

type handshake struct {
	Addr resource.Addr
}

func (h *handshake) GetAddr() resource.Addr {
	return h.Addr
}
