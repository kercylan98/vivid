package manager

import (
	"github.com/kercylan98/vivid/pkg/vivid"
	"github.com/kercylan98/vivid/src/resource"
)

var _ vivid.ManagerConnRegister = (*connRegister)(nil)

type connRegister struct {
	connections    map[resource.Addr]vivid.Conn
	readerBuilder  vivid.ConnReaderBuilder
	decoderBuilder vivid.DecoderBuilder
}

func (r *connRegister) Handle(server vivid.Server) {
	r.connections = make(map[resource.Addr]vivid.Conn)

	for conn := range server.GetConnChannel() {
		go r.handle(conn)
	}
}

func (r *connRegister) handle(conn vivid.Conn) {
	var c = make(chan vivid.Envelope, 1)
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
			handshakeMessage, ok := envelope.(vivid.ConnHandshakeMessage)
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
