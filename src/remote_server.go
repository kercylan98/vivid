package vivid

import (
	"errors"
	"github.com/kercylan98/vivid/src/internal/protobuf/protobuf"
	"sync/atomic"
)

var _ protobuf.VividServiceServer = (*remoteServer)(nil)

func newRemoteServer(address string) *remoteServer {
	return &remoteServer{
		address: address,
	}
}

type remoteServer struct {
	*protobuf.UnimplementedVividServiceServer
	manager *remoteStreamManager
	address string
	ready   atomic.Bool
}

func (s *remoteServer) setManager(manager *remoteStreamManager) {
	s.manager = manager
	s.ready.Store(true)
}

func (s *remoteServer) OpenMessageStream(conn protobuf.VividService_OpenMessageStreamServer) error {
	if !s.ready.Load() {
		return errors.New("remote server is not ready")
	}

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
