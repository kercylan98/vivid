package server

import (
	"github.com/kercylan98/vivid/pkg/vivid"
	"net"
)

var _ vivid.Server = (*server)(nil)

type server struct {
	options     vivid.ServerOptionsFetcher
	connChannel chan vivid.Conn
}

func (srv *server) GetConnChannel() <-chan vivid.Conn {
	return srv.connChannel
}

func (srv *server) Serve(listener net.Listener) error {
	srv.connChannel = make(chan vivid.Conn, srv.options.GetConnChannelSize())

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		srv.connChannel <- conn
	}
}
