package grpcnet

import (
	"github.com/kercylan98/vivid/ecology/grpc-net/grpcnet/internal/stream"
	"github.com/kercylan98/vivid/pkg/vivid/processor"
	"google.golang.org/grpc"
	"net"
)

var _ processor.RPCServer = (*server)(nil)

func newServer() *server {
	return &server{
		c: make(chan processor.RPCConn, 1024),
	}
}

type server struct {
	stream.UnimplementedStreamServer
	srv *grpc.Server
	c   chan processor.RPCConn
}

func (s *server) Serve(listen net.Listener) error {
	s.srv = grpc.NewServer()
	stream.RegisterStreamServer(s.srv, s)
	defer func() {
		s.srv = nil
	}()
	return s.srv.Serve(listen)
}

func (s *server) Stop() error {
	if s.srv == nil {
		return nil
	}

	s.srv.GracefulStop()
	return nil
}

func (s *server) Listen() <-chan processor.RPCConn {
	return s.c
}

func (s *server) ActorStreaming(stream grpc.BidiStreamingServer[stream.Message, stream.Message]) error {
	c := newServerConn(stream)
	s.c <- c
	select {
	case <-c.Context().Done():
		return c.context.Err()
	}
}
