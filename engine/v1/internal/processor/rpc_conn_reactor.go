package processor

import "github.com/kercylan98/vivid/engine/v1/processor"

type RPCConnReactor interface {
    OnMessage(conn processor.RPCConn, data []byte)
}

type RPCConnReactorFN func(conn processor.RPCConn, data []byte)

func (fn RPCConnReactorFN) OnMessage(conn processor.RPCConn, data []byte) {
    fn(conn, data)
}

type RPCConnReactorProvider interface {
    Provide() RPCConnReactor
}

type RPCConnReactorProviderFN func() RPCConnReactor

func (fn RPCConnReactorProviderFN) Provide() RPCConnReactor {
    return fn()
}
