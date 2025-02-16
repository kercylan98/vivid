package vivid

import (
	"github.com/kercylan98/vivid/src/internal/protobuf/protobuf"
)

var _ remoteStream = (*remoteStreamImpl)(nil)

type remoteGRpcStream interface {
	Send(message *protobuf.Message) error
	Recv() (*protobuf.Message, error)
}

type remoteStream interface {
	remoteGRpcStream
	isOpener() bool
	close()
	bindAddr(addr Addr)
	getAddr() Addr
	getCodec() Codec
}

func newRemoteStream(manager *remoteStreamManager, stream remoteGRpcStream) remoteStream {
	s := &remoteStreamImpl{manager: manager, codec: manager.processManager.getCodecProvider().Provide()}
	switch stream.(type) {
	case protobuf.VividService_OpenMessageStreamServer:
		s.client = stream.(protobuf.VividService_OpenMessageStreamServer)
	case protobuf.VividService_OpenMessageStreamClient:
		s.server = stream.(protobuf.VividService_OpenMessageStreamClient)
	default:
		panic("invalid remoteStream")
	}
	return s
}

type remoteStreamImpl struct {
	addr    Addr
	manager *remoteStreamManager
	client  protobuf.VividService_OpenMessageStreamServer // 服务端收到的客户端连接
	server  protobuf.VividService_OpenMessageStreamClient // 客户端打开的服务端连接
	codec   Codec
}

func (s *remoteStreamImpl) bindAddr(addr Addr) {
	s.addr = addr
}

func (s *remoteStreamImpl) getAddr() Addr {
	return s.addr
}

func (s *remoteStreamImpl) Send(message *protobuf.Message) error {
	if s.client != nil {
		return s.client.Send(message)
	} else {
		return s.server.Send(message)
	}
}

func (s *remoteStreamImpl) Recv() (*protobuf.Message, error) {
	if s.client != nil {
		return s.client.Recv()
	} else {
		return s.server.Recv()
	}
}

func (s *remoteStreamImpl) close() {
	if s.isOpener() {
		_ = s.server.CloseSend()
	}
	if s.addr == "" {
		return
	}
	s.manager.unbindRemoteStream(s.addr, s)
}

func (s *remoteStreamImpl) isOpener() bool {
	return s.server != nil
}

func (s *remoteStreamImpl) getCodec() Codec {
	return s.codec
}
