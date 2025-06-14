package processor

import (
	"context"
	processor2 "github.com/kercylan98/vivid/core/vivid/processor"
	"math/rand/v2"
	"net"
	"sync"
	"time"

	"github.com/kercylan98/go-log/log"
)

// NewRPCServer 创建新的 RPC 服务器实例。
// 会验证配置的完整性并设置合理的默认值。
// config 参数包含服务器运行所需的所有配置信息。
// 返回配置完整的 RPC 服务器实例。
func NewRPCServer(config *RPCServerConfiguration) *RPCServer {
	if config == nil {
		panic("RPC server configuration cannot be nil")
	}

	// 设置默认日志记录器
	if config.Logger == nil {
		config.Logger = log.GetDefault()
	}

	// 验证必填配置项
	if config.Server == nil {
		panic("RPC server core implementation is required")
	}
	if config.Serializer == nil {
		panic("RPC server serializer is required")
	}
	if config.ReactorProvider == nil {
		panic("RPC server reactor provider is required")
	}
	if config.Network == "" {
		panic("RPC server network type is required")
	}
	if config.AdvertisedAddress == "" {
		panic("RPC server advertised address is required")
	}
	if config.BindAddress == "" {
		panic("RPC server bind address is required")
	}

	return &RPCServer{
		config:  *config,
		remotes: make(map[string][]processor2.RPCConn),
		reactor: config.ReactorProvider.Provide(),
	}
}

// RPCServer RPC 服务器实现。
// 负责处理远程连接的建立、维护和消息路由。
// 支持多客户端并发连接和优雅关闭。
type RPCServer struct {
	config      RPCServerConfiguration          // 服务器配置
	context     context.Context                 // 运行上下文，用于控制服务器生命周期
	cancel      context.CancelFunc              // 取消函数，用于停止服务器
	remotes     map[string][]processor2.RPCConn // 远程连接映射，key为地址，value为连接列表
	remotesLock sync.RWMutex                    // 远程连接锁，保护并发访问
	reactor     RPCConnReactor                  // 连接反应器，处理消息事件
}

// SetContext 设置服务器运行上下文。
// 必须在调用 Run 方法之前设置，否则服务器无法正常停止。
// ctx 参数指定服务器的运行上下文。
func (srv *RPCServer) SetContext(ctx context.Context) {
	srv.context, srv.cancel = context.WithCancel(ctx)
}

// Run 启动 RPC 服务器并开始监听连接。
// 此方法会阻塞直到上下文被取消或发生错误。
// 返回启动过程中遇到的错误，正常停止时返回 nil。
func (srv *RPCServer) Run() error {
	// 确保上下文已设置
	if srv.context == nil {
		srv.Logger().Warn("RPC server context not set, creating default context")
		srv.context, srv.cancel = context.WithCancel(context.Background())
	}

	lis, err := net.Listen(srv.config.Network, srv.config.BindAddress)
	if err != nil {
		return err
	}

	srv.Logger().Info("RPC server listening",
		log.String("network", srv.config.Network),
		log.String("address", srv.config.BindAddress))

	go func() {
		defer func() {
			if r := recover(); r != nil {
				srv.Logger().Error("RPC server core panic", log.Any("panic", r))
			}
		}()

		if err := srv.config.Server.Serve(lis); err != nil {
			srv.Logger().Error("RPC server core error", log.Err(err))
		}
	}()

	defer func() {
		if err := lis.Close(); err != nil {
			srv.Logger().Error("Failed to close listener", log.Err(err))
		}
		srv.Logger().Info("RPC server stopped")
	}()

	for {
		select {
		case <-srv.context.Done():
			srv.Logger().Info("RPC server stopping due to context cancellation")
			return nil
		case conn := <-srv.config.Server.Listen():
			go srv.onConnected(conn)
		}
	}
}

// Stop 停止 RPC 服务器。
// 此方法是异步的，会触发上下文取消，实际停止由 Run 方法完成。
func (srv *RPCServer) Stop() {
	if srv.cancel != nil {
		srv.Logger().Info("Stopping RPC server")
		srv.cancel()
	}
}

func (srv *RPCServer) GetConn(address string) processor2.RPCConn {
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

func (srv *RPCServer) onConnected(conn processor2.RPCConn) {
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

		if err = conn.Close(); err != nil {
			srv.Logger().Error("onConnected", log.Err(err))
		}
	}()

	var packet []byte
	for {
		select {
		case <-srv.context.Done():
			return
		case <-time.After(time.Minute): // 一分钟超时
			srv.Logger().Error("onConnected", log.String("event", "Timeout"))
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

func (srv *RPCServer) onWaitHandshake(conn processor2.RPCConn) (processor2.RPCHandshake, error) {
	packet, err := conn.Recv()
	if err != nil {
		srv.Logger().Error("onWaitHandshake", log.Err(err))
		return nil, err
	}

	_, data := unpackRPCMessage(packet)
	handshake := processor2.NewRPCHandshake()
	if err := handshake.Unmarshal(data); err != nil {
		srv.Logger().Error("onWaitHandshake", log.String("event", "Unmarshal"), log.Err(err))
		return nil, err
	}

	reply, err := processor2.NewRPCHandshakeWithAddress(srv.config.AdvertisedAddress).Marshal()
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
