package grpcnet

import (
	"context"
	"github.com/kercylan98/vivid/grpc-net/grpcnet/internal/stream"
	"github.com/kercylan98/vivid/pkg/vivid/processor"
	"google.golang.org/grpc"
)

var _ processor.RPCConn = (*serverConn)(nil)

func newServerConn(stream grpc.BidiStreamingServer[stream.Message, stream.Message]) *serverConn {
	c := &serverConn{
		stream: stream,
	}
	c.context, c.cancel = context.WithCancel(stream.Context())
	return c
}

func newClientConn(stream grpc.BidiStreamingClient[stream.Message, stream.Message]) *clientConn {
	c := &clientConn{
		stream: stream,
	}
	return c
}

type serverConn struct {
	context context.Context
	cancel  context.CancelFunc
	stream  grpc.BidiStreamingServer[stream.Message, stream.Message]
}

func (c *serverConn) Send(bytes []byte) error {
	return c.stream.Send(&stream.Message{Data: bytes})
}

func (c *serverConn) Recv() ([]byte, error) {
	m, err := c.stream.Recv()
	if err != nil {
		return nil, err
	}
	return m.Data, nil
}

func (c *serverConn) Close() error {
	c.cancel()
	return nil
}

func (c *serverConn) Context() context.Context {
	return c.context
}

type clientConn struct {
	stream grpc.BidiStreamingClient[stream.Message, stream.Message]
}

func (c *clientConn) Send(bytes []byte) error {
	return c.stream.Send(&stream.Message{Data: bytes})
}

func (c *clientConn) Recv() ([]byte, error) {
	m, err := c.stream.Recv()
	if err != nil {
		return nil, err
	}
	return m.Data, nil
}

func (c *clientConn) Close() error {
	return c.stream.CloseSend()
}
