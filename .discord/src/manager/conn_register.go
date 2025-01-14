package manager

import (
	vivid2 "github.com/kercylan98/vivid/.discord/pkg/vivid"
	"github.com/kercylan98/vivid/.discord/src/resource"
)

var _ vivid2.ManagerConnRegister = (*connRegister)(nil)

type connRegister struct {
	connections    map[resource.Addr]vivid2.Conn
	readerBuilder  vivid2.ConnReaderBuilder
	decoderBuilder vivid2.DecoderBuilder
}

func (r *connRegister) Handle(server vivid2.Server) {
	r.connections = make(map[resource.Addr]vivid2.Conn)

	for conn := range server.GetConnChannel() {
		go r.handle(conn)
	}
}

func (r *connRegister) handle(conn vivid2.Conn) {
	var c = make(chan vivid2.Envelope, 1)
	defer func() {
		close(c)
		if err := conn.Close(); err != nil {
			panic(err)
		}
	}()
	var handshake bool
	go r.readerBuilder.Build(conn, r.decoderBuilder).Read(c)
	for envelope := range c {
		if !handshake {
			handshake = true
			handshakeMessage, ok := envelope.(vivid2.ConnHandshakeMessage)
			if !ok {
				break
			}

			if _, exist := r.connections[handshakeMessage.GetAddr()]; exist {
				panic("duplicate connection")
			}

			r.connections[handshakeMessage.GetAddr()] = conn
			continue
		}

	}
}
