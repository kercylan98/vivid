package actor

import (
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/chain"
	"github.com/kercylan98/vivid/internal/guard"
	"github.com/kercylan98/vivid/internal/messages"
	"github.com/kercylan98/vivid/internal/messages/messagecodecs"
	metricsActor "github.com/kercylan98/vivid/internal/metrics"
	"github.com/kercylan98/vivid/internal/remoting"
	"github.com/kercylan98/vivid/internal/serialization"
	"github.com/kercylan98/vivid/internal/virtual"
	"github.com/kercylan98/vivid/pkg/metrics"
)

var systemChains = &_systemChains{}

type _systemChains struct{}

func (c *_systemChains) initializeCodec(system *System) chain.Chain {
	return chain.ChainFN(func() (err error) {
		system.codec = serialization.NewVividCodec(system.options.RemotingCodec)

		// 注册系统消息
		registry := newMessageRegistry(system.codec).SetClass("SYS")

		registry.RegisterMessage("*vivid.Error", new(vivid.Error))
		registry.RegisterMessageWithEncoderAndDecoder("*vivid.OnLaunch", new(vivid.OnLaunch), messagecodecs.OnLaunchEncoder(), messagecodecs.OnLaunchDecoder())
		registry.RegisterMessageWithEncoderAndDecoder("*vivid.OnKill", new(vivid.OnKill), messagecodecs.OnKillEncoder(), messagecodecs.OnKillDecoder())
		registry.RegisterMessageWithEncoderAndDecoder("*vivid.OnKilled", new(vivid.OnKilled), messagecodecs.OnKilledEncoder(), messagecodecs.OnKilledDecoder())
		registry.RegisterMessageWithEncoderAndDecoder("*vivid.Pong", new(vivid.Pong), messagecodecs.PongEncoder(), messagecodecs.PongDecoder())
		registry.RegisterMessageWithEncoderAndDecoder("*vivid.PipeResult", new(vivid.PipeResult), messagecodecs.PipeResultEncoder(), messagecodecs.PipeResultDecoder())
		registry.RegisterMessageWithEncoderAndDecoder("*messages.NoneArgsCommandMessage", new(messages.NoneArgsCommandMessage), messagecodecs.NoneArgsCommandMessageEncoder(), messagecodecs.NoneArgsCommandMessageDecoder())
		registry.RegisterMessageWithEncoderAndDecoder("*messages.PingMessage", new(messages.PingMessage), messagecodecs.PingMessageEncoder(), messagecodecs.PingMessageDecoder())
		registry.RegisterMessageWithEncoderAndDecoder("*messages.PongMessage", new(messages.PongMessage), messagecodecs.PongMessageEncoder(), messagecodecs.PongMessageDecoder())
		registry.RegisterMessageWithEncoderAndDecoder("*messages.WatchMessage", new(messages.WatchMessage), messagecodecs.WatchMessageEncoder(), messagecodecs.WatchMessageDecoder())
		registry.RegisterMessageWithEncoderAndDecoder("*messages.UnwatchMessage", new(messages.UnwatchMessage), messagecodecs.UnwatchMessageEncoder(), messagecodecs.UnwatchMessageDecoder())
		registry.RegisterMessageWithEncoderAndDecoder("*actor.SchedulerMessage", new(SchedulerMessage), messagecodecs.SchedulerMessageEncoder(), messagecodecs.SchedulerMessageDecoder())
		registry.RegisterMessage("*virtual.Identity", new(virtual.Identity))

		// 注册用户消息
		registry.SetClass("USER")
		for _, register := range system.options.MessageRegister {
			register.Register(registry)
		}
		return registry.Err()
	})
}

func (c *_systemChains) spawnGuardActor(system *System) chain.Chain {
	return chain.ChainFN(func() (err error) {
		system.Context, err = NewContext(system, nil, guard.NewActor(system.guardClosedSignal))
		if err != nil {
			return err
		}
		return nil
	})
}

func (c *_systemChains) initializeMetrics(system *System) chain.Chain {
	return chain.ChainFN(func() (err error) {
		if system.options.Metrics != nil {
			system.metrics = system.options.Metrics
		} else {
			system.metrics = metrics.NewDefaultMetrics()
		}
		if system.options.EnableMetrics {
			ma := metricsActor.NewActor(system.options.EnableMetricsUpdatedNotify)
			_, err = system.ActorOf(ma, vivid.WithActorName("@metrics"))
			return err
		}
		return nil
	})
}

func (c *_systemChains) initializeRemoting(system *System) chain.Chain {
	return chain.ChainFN(func() (err error) {
		if system.options.RemotingBindAddress == "" || system.options.RemotingAdvertiseAddress == "" {
			return nil
		}

		system.remotingServer = remoting.NewServerActor(
			system.options.Context,
			system.options.RemotingBindAddress,
			system.options.RemotingAdvertiseAddress,
			system.codec,
			system, // NetworkEnvelopHandler 实现
			*system.options.RemotingOptions,
		)
		system.options.Logger = system.options.Logger.With("addr", system.options.RemotingAdvertiseAddress)
		_, err = system.ActorOf(system.remotingServer, vivid.WithActorName("@remoting"))
		return err
	})
}

func (c *_systemChains) initializeVirtualCoordinator(system *System) chain.Chain {
	return chain.ChainFN(func() (err error) {
		if len(system.options.VirtualActorProviders) == 0 {
			return nil
		}
		coordinatorInjecter := virtual.NewCoordinatorActor(system)
		system.virtualCoordinator, err = coordinatorInjecter.Inject(system)
		return err
	})
}
