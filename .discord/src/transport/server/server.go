package server

import (
	vivid2 "github.com/kercylan98/vivid/.discord/pkg/vivid"
	"net"
)

var _ vivid2.Server = (*server)(nil)

type server struct {
	options     vivid2.ServerOptionsFetcher
	connChannel chan vivid2.Conn
}

func (srv *server) GetConnChannel() <-chan vivid2.Conn {
	return srv.connChannel
}

func (srv *server) Serve(listener net.Listener) error {
	srv.connChannel = make(chan vivid2.Conn, srv.options.GetConnChannelSize())

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		srv.connChannel <- conn
	}
}
