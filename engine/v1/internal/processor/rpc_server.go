package processor

import (
    "context"
    "github.com/kercylan98/go-log/log"
    "github.com/kercylan98/vivid/engine/v1/processor"
    "math/rand/v2"
    "net"
    "sync"
)

func NewRPCServer(config *RPCServerConfiguration) *RPCServer {
    if config.Logger == nil {
        config.Logger = log.GetDefault()
    }

    return &RPCServer{
        config:  *config,
        remotes: make(map[string][]processor.RPCConn),
        reactor: config.ReactorProvider.Provide(),
    }
}

type RPCServer struct {
    config      RPCServerConfiguration         // 配置
    context     context.Context                // 上下文
    cancel      context.CancelFunc             // 取消函数
    remotes     map[string][]processor.RPCConn // 远程连接
    remotesLock sync.RWMutex                   // 远程连接锁
    reactor     RPCConnReactor                 // 连接反应器
}

func (srv *RPCServer) Run() error {
    lis, err := net.Listen(srv.config.Network, srv.config.BindAddress)
    if err != nil {
        return err
    }

    go func() {
        if err := srv.config.Server.Serve(lis); err != nil {
            panic(err)
        }
    }()

    defer func() {
        if err := lis.Close(); err != nil {
            srv.Logger().Error("Run", log.Err(err))
        }
    }()

    for {
        select {
        case <-srv.context.Done():
            return nil
        case conn := <-srv.config.Server.Listen():
            go srv.onConnected(conn)
        }
    }
}

func (srv *RPCServer) Stop() {
    srv.cancel()
}

func (srv *RPCServer) GetConn(address string) processor.RPCConn {
    srv.remotesLock.RLock()
    defer srv.remotesLock.RUnlock()

    connections, found := srv.remotes[address]
    if !found {
        return nil
    }

    return connections[rand.IntN(len(connections))]
}

func (srv *RPCServer) Logger() log.Logger {
    return srv.config.Logger
}

func (srv *RPCServer) onConnected(conn processor.RPCConn) {
    handshake, err := srv.onWaitHandshake(conn)
    if err != nil {
        srv.Logger().Error("onConnected", log.Err(err))
        if err = conn.Close(); err != nil {
            srv.Logger().Error("onConnected", log.Err(err))
        }
        return
    }

    advertisedAddress := handshake.GetAddress()

    srv.remotesLock.Lock()
    srv.remotes[advertisedAddress] = append(srv.remotes[advertisedAddress], conn)
    srv.remotesLock.Unlock()

    defer func() {
        srv.remotesLock.Lock()
        for idx, rpcConn := range srv.remotes[advertisedAddress] {
            if rpcConn == conn {
                srv.remotes[advertisedAddress] = append(srv.remotes[advertisedAddress][:idx], srv.remotes[advertisedAddress][idx+1:]...)
                break
            }
        }
        if len(srv.remotes[advertisedAddress]) == 0 {
            delete(srv.remotes, advertisedAddress)
        }
        srv.remotesLock.Unlock()
    }()

    var packet []byte

    for {
        select {
        case <-srv.context.Done():
            return
        default:
            packet, err = conn.Recv()
            if err != nil {
                srv.Logger().Error("onConnected", log.Err(err))
                if err = conn.Close(); err != nil {
                    srv.Logger().Error("onConnected", log.Err(err))
                }
                return
            }

            srv.reactor.OnMessage(conn, packet)
        }
    }
}

func (srv *RPCServer) onWaitHandshake(conn processor.RPCConn) (processor.RPCHandshake, error) {
    packet, err := conn.Recv()
    if err != nil {
        srv.Logger().Error("onWaitHandshake", log.Err(err))
        return nil, err
    }

    _, data := unpackRPCMessage(packet)
    handshake := processor.NewRPCHandshake()
    if err := handshake.Unmarshal(data); err != nil {
        srv.Logger().Error("onWaitHandshake", log.String("event", "Unmarshal"), log.Err(err))
        return nil, err
    }

    reply, err := processor.NewRPCHandshakeWithAddress(srv.config.AdvertisedAddress).Marshal()
    if err != nil {
        srv.Logger().Error("onWaitHandshake", log.String("event", "Marshal"), log.Err(err))
        return nil, err
    }

    if err = conn.Send(reply); err != nil {
        srv.Logger().Error("onWaitHandshake", log.String("event", "Send"), log.Err(err))
        return nil, err
    }

    return handshake, nil
}
