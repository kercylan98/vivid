package vivid

import (
	"github.com/kercylan98/chrono/timing"
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/src/internal/protobuf/protobuf"
	"google.golang.org/grpc"
)

var (
	_ actorSystemInternal = (*actorSystemInternalImpl)(nil)
)

type actorSystemInternal interface {
	setConfig(config ActorSystemOptionsFetcher)

	getConfig() ActorSystemOptionsFetcher

	getProcessManager() processManager

	getTimingWheel() timing.Wheel

	initRemote() error

	initProcessManager()

	writeInitLog(args ...any)
}

func newActorSystemInternal(system ActorSystem, config ActorSystemOptionsFetcher) actorSystemInternal {
	return &actorSystemInternalImpl{
		ActorSystem: system,
		config:      config,
		timingWheel: timing.New(timing.ConfiguratorFn(func(config timing.Configuration) {
			config.WithSize(50)
		})),
	}
}

type actorSystemInternalImpl struct {
	ActorSystem
	config         ActorSystemOptionsFetcher
	processManager processManager
	timingWheel    timing.Wheel
	grpcServer     *grpc.Server
	remoteServer   *remoteServer
}

func (a *actorSystemInternalImpl) getConfig() ActorSystemOptionsFetcher {
	return a.config
}

func (a *actorSystemInternalImpl) setConfig(config ActorSystemOptionsFetcher) {
	a.config = config
}

func (a *actorSystemInternalImpl) getProcessManager() processManager {
	return a.processManager
}

func (a *actorSystemInternalImpl) getTimingWheel() timing.Wheel {
	return a.timingWheel
}

func (a *actorSystemInternalImpl) initRemote() error {
	listener := a.config.FetchListener()
	if listener == nil {
		return nil
	}

	a.remoteServer = newRemoteServer(listener.Addr().String())
	a.grpcServer = grpc.NewServer()
	a.grpcServer.RegisterService(&protobuf.VividService_ServiceDesc, a.remoteServer)

	go func() {
		if err := a.grpcServer.Serve(listener); err != nil {
			panic(err)
		}
	}()

	return nil
}

func (a *actorSystemInternalImpl) initProcessManager() {
	// TODO: 不应该使用 localhost 作为本地地址，因为远程通讯时候，本地 ActorSystem 是可以主动访问到远程 ActorSystem 的。这时候记录的地址会是 localhost，如果有多个，那么将会出现问题。暂时没有更好的解决方案，后续需要考虑如何解决这个问题。
	var addr = "localhost"
	if listener := a.getConfig().FetchListener(); listener != nil {
		addr = listener.Addr().String()
	}
	a.processManager = newProcessManager(addr, a.getConfig().FetchCodec(), log.ProviderFn(func() log.Logger {
		return a.getConfig().FetchLogger()
	}))

	if a.remoteServer != nil {
		a.remoteServer.setManager(a.processManager.getRemoteStreamManager())
	}
}

func (a *actorSystemInternalImpl) writeInitLog(args ...any) {
	a.Logger().Info("Starting ActorSystem", args...)
}
