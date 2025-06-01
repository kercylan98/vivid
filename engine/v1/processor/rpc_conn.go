package processor

type RPCConn interface {
    Send(bytes []byte) error
    Recv() ([]byte, error)
    Close() error
}

type RPCConnProvider interface {
    Provide(address string) (RPCConn, error)
}

type RPCConnProviderFN func(address string) (RPCConn, error)

func (fn RPCConnProviderFN) Provide(address string) (RPCConn, error) {
    return fn(address)
}
