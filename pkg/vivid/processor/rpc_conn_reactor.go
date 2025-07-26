package processor

import "github.com/kercylan98/vivid/pkg/serializer"

type RPCConnReactor interface {
	OnMessage(conn RPCConn, serializer serializer.NameSerializer, data []byte)
}

type RPCConnReactorFN func(conn RPCConn, data []byte)

func (fn RPCConnReactorFN) OnMessage(conn RPCConn, serializer serializer.NameSerializer, data []byte) {
	fn(conn, data)
}

type RPCConnReactorProvider interface {
	Provide() RPCConnReactor
}

type RPCConnReactorProviderFN func() RPCConnReactor

func (fn RPCConnReactorProviderFN) Provide() RPCConnReactor {
	return fn()
}
