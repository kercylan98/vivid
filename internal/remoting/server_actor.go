package remoting

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/utils"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/sugar"
)

var (
	_                    vivid.Actor = (*ServerActor)(nil)
	startAcceptorMessage             = new(startAcceptor)
)

type ServerActorOptions struct {
	ListenerCreatedHandler func(listener net.Listener)
}

type startAcceptor struct{}

// NewServerActor 创建新的服务器
func NewServerActor(bindAddr string, advertiseAddr string, codec vivid.Codec, envelopHandler NetworkEnvelopHandler, options ServerActorOptions) *ServerActor {
	sa := &ServerActor{
		options:           options,
		bindAddr:          bindAddr,
		advertiseAddr:     advertiseAddr,
		codec:             codec,
		envelopHandler:    envelopHandler,
		acceptConnections: make(map[string]*tcpConnectionActor),
		backoff:           utils.NewExponentialBackoff(100*time.Millisecond, 10*time.Second, 2, true),
	}
	sa.remotingMailboxCentralWG.Add(1)
	return sa
}

// ServerActor 管理TCP服务器
type ServerActor struct {
	options                  ServerActorOptions             // 服务器选项
	bindAddr                 string                         // TCP服务器绑定的本地监听地址（如 "0.0.0.0:8080"），只绑定本地，不对外暴露
	advertiseAddr            string                         // 对外宣称的服务地址（如 "public.ip:port"），用于服务注册和远程节点发现
	acceptorRef              vivid.ActorRef                 // 当前正在维护 net.Listener 和负责被动接收和管理新 TCP 连接的 Acceptor Actor 的引用
	backoff                  *utils.ExponentialBackoff      // 指数退避器，用于监听端口失败时的重试延迟策略，防止过于频繁重试导致资源浪费
	backoffTimer             *time.Timer                    // 指数退避重试定时器，用于重试启动服务器监听
	acceptConnections        map[string]*tcpConnectionActor // 当前已建立并被服务器管理的连接集合，key为连接唯一标识
	codec                    vivid.Codec                    // 消息编解码器，实现消息的序列化与反序列化
	envelopHandler           NetworkEnvelopHandler          // 网络消息处理器，处理接收到的远程消息
	remotingMailboxCentral   *MailboxCentral                // 远程邮箱中心，用于转发和分发网络层消息的核心模块
	remotingMailboxCentralWG sync.WaitGroup                 // 用于等待远程邮箱中心初始化完成，保证远程相关操作在其准备好后再进行
}

func (s *ServerActor) OnReceive(ctx vivid.ActorContext) {
	switch message := ctx.Message().(type) {
	case *vivid.OnLaunch:
		s.onLaunch(ctx)
	case *tcpConnectionActor:
		s.onConnection(ctx, message)
	case *vivid.OnKill:
		s.onKill(ctx, message)
	case *vivid.OnKilled:
		s.onKilled(ctx, message)
	case *startAcceptor:
		s.onStartAcceptor(ctx)
	}
}

func (s *ServerActor) onStartAcceptor(ctx vivid.ActorContext) {
	// 避免重复启动
	if s.acceptorRef != nil {
		ctx.Logger().Warn("server acceptor already started, ignore", log.String("bind_addr", s.bindAddr))
		return
	}

	var (
		addr     *net.TCPAddr
		err      error
		listener net.Listener
	)

	addr, err = net.ResolveTCPAddr("tcp", s.bindAddr)
	if err == nil {
		listener, err = net.ListenTCP("tcp", addr)
	}

	if err != nil {
		delay := s.backoff.Next()
		ctx.Logger().Warn("server listener resolve address failed, restart later", log.String("bind_addr", s.bindAddr), log.Duration("delay", delay), log.Any("err", err))
		s.backoffTimer = time.AfterFunc(delay, func() {
			ctx.TellSelf(startAcceptorMessage)
		})
		return
	}

	// 重置及清理重试相关状态
	s.backoff.Reset()
	if s.backoffTimer != nil {
		s.backoffTimer.Stop()
		s.backoffTimer = nil
	}
	ctx.Logger().Info("server listener started", log.String("bind_addr", listener.Addr().String()))

	// 安全的 Unwrap， ServerAcceptActor 未实现 vivid.PrelaunchActor 接口，不会发生错误
	acceptor := newServerAcceptActor(listener, s.advertiseAddr, s.envelopHandler, s.codec)
	s.acceptorRef = ctx.ActorOf(acceptor, vivid.WithActorName("acceptor")).Unwrap()
	if s.options.ListenerCreatedHandler != nil {
		s.options.ListenerCreatedHandler(listener)
	}
}

func (s *ServerActor) onLaunch(ctx vivid.ActorContext) {
	// 可能存在 Actor 还未启动完成旧投递网络消息，因此需要使用 WaitGroup 等待初始化完成
	s.remotingMailboxCentral = newMailboxCentral(ctx.Ref(), ctx, s.codec)
	s.remotingMailboxCentralWG.Done()

	// 投递 Acceptor 作为启动消息，实现重试启动
	ctx.TellSelf(startAcceptorMessage)
}

func (s *ServerActor) onConnection(ctx vivid.ActorContext, connection *tcpConnectionActor) {
	prefix := "dial"
	if !connection.client {
		// 维护当前已建立并被服务器管理的连接集合
		prefix = "accept"
	}
	// 连接至服务端的无需绑定，客户端自行维护连接，不进行复用
	ref := ctx.ActorOf(connection, vivid.WithActorName(fmt.Sprintf("%s-%s", prefix, connection.conn.RemoteAddr().String()))).
		Then(func(rc sugar.ResultContainer[vivid.ActorRef], ar vivid.ActorRef) *sugar.Result[vivid.ActorRef] {
			ctx.Reply(nil)
			return rc.Ok(ar)
		}).
		Else(func(rc sugar.ResultContainer[vivid.ActorRef], err error) *sugar.Result[vivid.ActorRef] {
			return rc.Error(err)
		}).
		Unwrap()

	if !connection.client {
		s.acceptConnections[ref.GetPath()] = connection
	}
}

func (s *ServerActor) GetRemotingMailboxCentral() *MailboxCentral {
	s.remotingMailboxCentralWG.Wait()
	return s.remotingMailboxCentral
}

func (s *ServerActor) onKill(ctx vivid.ActorContext, _ *vivid.OnKill) {
	s.remotingMailboxCentral.Close()
	for _, actor := range s.acceptConnections {
		if err := actor.Close(); err != nil {
			ctx.Logger().Warn("server accept connect close fail",
				log.String("advertise_addr", actor.advertiseAddr),
				log.Any("err", err),
			)
		}
	}

	if s.backoffTimer != nil {
		s.backoffTimer.Stop()
		s.backoffTimer = nil
	}
}

func (s *ServerActor) onKilled(ctx vivid.ActorContext, message *vivid.OnKilled) {
	ctx.Logger().Debug("server actor killed", log.Bool("self", ctx.Ref().Equals(message.Ref)), log.String("target", message.Ref.GetPath()))

	switch {
	case s.acceptConnections[message.Ref.GetPath()] != nil:
		// 如果是维护的连接销毁，从集合中移除
		tcpConn := s.acceptConnections[message.Ref.GetPath()]
		delete(s.acceptConnections, message.Ref.GetPath())
		if err := tcpConn.Close(); err != nil {
			ctx.Logger().Warn("server accept connect close fail",
				log.String("advertise_addr", tcpConn.advertiseAddr),
				log.Any("err", err),
			)
		}

	case message.Ref.Equals(s.acceptorRef):
		// Acceptor 被终止，尝试重启
		// 假如是此 Actor 的终止导致其终止，那么该消息会被屏蔽，因为 Actor 已经非正常状态
		s.acceptorRef = nil
		ctx.TellSelf(startAcceptorMessage)
	}
}
