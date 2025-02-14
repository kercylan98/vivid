package vivid

import (
	"github.com/kercylan98/vivid/src/internal/protobuf/protobuf"
)

var _ protobuf.VividServiceServer = (*Server)(nil)

type Server struct {
	*protobuf.UnimplementedVividServiceServer
	manager *remoteStreamManager
	address string
}

func (s *Server) OpenMessageStream(conn protobuf.VividService_OpenMessageStreamServer) error {
	stream := newRemoteStream(s.manager, conn)
	if handshake, err := s.manager.waitRemoteStreamHandshake(stream); err != nil {
		return err
	} else {
		stream.bindAddr(handshake.Address)
		s.manager.bindRemoteStream(handshake.Address, stream)
		s.manager.startListenRemoteStreamMessage(stream)
		return nil
	}
}
