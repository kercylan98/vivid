package actor

import (
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/chain"
	"github.com/kercylan98/vivid/internal/clusterstore"
	"github.com/kercylan98/vivid/internal/gossip"
	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/internal/gossip/gossipmessages"
	"github.com/kercylan98/vivid/internal/gossip/memberlist"
	"github.com/kercylan98/vivid/internal/gossip/versionvector"
	"github.com/kercylan98/vivid/internal/guard"
	"github.com/kercylan98/vivid/internal/messages"
	"github.com/kercylan98/vivid/internal/messages/messagecodecs"
	metricsActor "github.com/kercylan98/vivid/internal/metrics"
	"github.com/kercylan98/vivid/internal/remoting"
	"github.com/kercylan98/vivid/internal/serialization"
	"github.com/kercylan98/vivid/internal/virtual"
	"github.com/kercylan98/vivid/pkg/metrics"
	"github.com/kercylan98/vivid/pkg/ves"
)

const (
	remotingActorName = "@remoting"
	metricsActorName  = "@metrics"
)

// 停止优先级, 值越大越先停止（越靠后越先停止）
const (
	metricsStopPriority = iota
	remotingStopPriority
	gossipStopPriority
)

func init() {
	messagecodecs.InitActorRefParser(func(ref string) (vivid.ActorRef, error) {
		return ParseRef(ref)
	})
}

func newSystemChains(availableSignal <-chan struct{}) *systemChains {
	return &systemChains{
		availableSignal: availableSignal,
	}
}

type systemChains struct {
	availableSignal <-chan struct{}
}

func (c *systemChains) initializeCodec(system *System) chain.Chain {
	return chain.ChainFN(func() (err error) {
		system.codec = serialization.NewVividCodec(system.options.RemotingCodec, system.options.RemotingOptions.DecodeLimits)

		// 注册系统消息
		registry := newMessageRegistry(system.codec).SetClass("SYS")

		registry.RegisterMessage("*vivid.Error", new(vivid.Error))
		registry.RegisterMessage("*actor.Ref", new(Ref))
		registry.RegisterMessage("*actor.AgentRef", new(AgentRef))
		registry.RegisterMessage("*clusterstore.RaftWire", new(clusterstore.RaftWire))
		registry.RegisterMessage("*clusterstore.RaftControlRequest", new(clusterstore.RaftControlRequest))
		registry.RegisterMessage("*clusterstore.RaftControlResponse", new(clusterstore.RaftControlResponse))
		registry.RegisterMessageWithEncoderAndDecoder("*vivid.OnLaunch", new(vivid.OnLaunch), messagecodecs.OnLaunchEncoder(), messagecodecs.OnLaunchDecoder())
		registry.RegisterMessageWithEncoderAndDecoder("*vivid.OnKill", new(vivid.OnKill), messagecodecs.OnKillEncoder(), messagecodecs.OnKillDecoder())
		registry.RegisterMessageWithEncoderAndDecoder("*vivid.OnKilled", new(vivid.OnKilled), messagecodecs.OnKilledEncoder(), messagecodecs.OnKilledDecoder())
		registry.RegisterMessageWithEncoderAndDecoder("*vivid.Pong", new(vivid.Pong), messagecodecs.PongEncoder(), messagecodecs.PongDecoder())
		registry.RegisterMessageWithEncoderAndDecoder("*vivid.PipeResult", new(vivid.PipeResult), messagecodecs.PipeResultEncoder(), messagecodecs.PipeResultDecoder())
		registry.RegisterMessageWithEncoderAndDecoder("*vivid.Heartbeat", new(vivid.Heartbeat), messagecodecs.HeartbeatEncoder(), messagecodecs.HeartbeatDecoder())
		registry.RegisterMessageWithEncoderAndDecoder("*messages.NoneArgsCommandMessage", new(messages.NoneArgsCommandMessage), messagecodecs.NoneArgsCommandMessageEncoder(), messagecodecs.NoneArgsCommandMessageDecoder())
		registry.RegisterMessageWithEncoderAndDecoder("*messages.PingMessage", new(messages.PingMessage), messagecodecs.PingMessageEncoder(), messagecodecs.PingMessageDecoder())
		registry.RegisterMessageWithEncoderAndDecoder("*messages.PongMessage", new(messages.PongMessage), messagecodecs.PongMessageEncoder(), messagecodecs.PongMessageDecoder())
		registry.RegisterMessageWithEncoderAndDecoder("*messages.WatchMessage", new(messages.WatchMessage), messagecodecs.WatchMessageEncoder(), messagecodecs.WatchMessageDecoder())
		registry.RegisterMessageWithEncoderAndDecoder("*messages.UnwatchMessage", new(messages.UnwatchMessage), messagecodecs.UnwatchMessageEncoder(), messagecodecs.UnwatchMessageDecoder())
		registry.RegisterMessage("*virtual.Identity", new(virtual.Identity))
		routedCodec := virtual.NewRoutedDeliveryCodec(system.codec)
		registry.RegisterMessageWithEncoderAndDecoder("*virtual.RoutedDelivery", new(virtual.RoutedDelivery), routedCodec, routedCodec)
		registry.RegisterMessage("*virtual.DeliveryResult", new(virtual.DeliveryResult))
		registry.RegisterMessage("*virtual.ActivationRequest", new(virtual.ActivationRequest))
		registry.RegisterMessage("*virtual.ActivationResult", new(virtual.ActivationResult))
		registry.RegisterMessage("*virtual.DiscoveryRequest", new(virtual.DiscoveryRequest))
		registry.RegisterMessage("*virtual.DiscoveryResult", new(virtual.DiscoveryResult))
		registry.RegisterMessageWithEncoderAndDecoder("*endpoint.Information", new(endpoint.Information), messagecodecs.GenericEncoder(), messagecodecs.GenericDecoder())
		registry.RegisterMessage("*memberlist.MemberList", new(memberlist.MemberList))
		registry.RegisterMessage("*versionvector.VersionVector", new(versionvector.VersionVector))
		registry.RegisterMessageWithEncoderAndDecoder("*gossipmessages.Ping", new(gossipmessages.Ping), messagecodecs.GenericEncoder(), messagecodecs.GenericDecoder())
		registry.RegisterMessageWithEncoderAndDecoder("*gossipmessages.Pong", new(gossipmessages.Pong), messagecodecs.GenericEncoder(), messagecodecs.GenericDecoder())
		registry.RegisterMessageWithEncoderAndDecoder("*gossipmessages.PongAck", new(gossipmessages.PongAck), messagecodecs.GenericEncoder(), messagecodecs.GenericDecoder())
		registry.RegisterMessageWithEncoderAndDecoder("*gossipmessages.JoinRejected", new(gossipmessages.JoinRejected), messagecodecs.GenericEncoder(), messagecodecs.GenericDecoder())

		// 注册用户消息
		registry.SetClass("USER")
		for _, register := range system.options.MessageRegister {
			register.Register(registry)
		}
		return registry.Err()
	})
}

func (c *systemChains) spawnGuardActor(system *System) chain.Chain {
	return chain.ChainFN(func() (err error) {
		ctx, err := NewContext(system, nil, guard.NewActor())
		if err != nil {
			return err
		}
		system.statusLock.Lock()
		system.Context = ctx
		system.statusLock.Unlock()
		return nil
	})
}

func (c *systemChains) initializeMetrics(system *System) chain.Chain {
	return chain.ChainFN(func() (err error) {
		if system.options.Metrics != nil {
			system.metrics = system.options.Metrics
		} else {
			system.metrics = metrics.NewDefaultMetrics()
		}
		if system.options.EnableMetrics {
			ma := metricsActor.NewActor(system.options.EnableMetricsUpdatedNotify)
			ref, err := system.ActorOf(ma, vivid.WithActorName(metricsActorName))
			if err != nil {
				return err
			}
			system.tell(true, system.Ref(), &guard.RegisterStopPriority{
				ActorRef: ref,
				Priority: metricsStopPriority,
			})
		}
		return nil
	})
}

func (c *systemChains) initializeRemoting(system *System) chain.Chain {
	return chain.ChainFN(func() (err error) {
		if system.options.RemotingBindAddress == "" || system.options.RemotingAdvertiseAddress == "" {
			return nil
		}

		remotingOptions := *system.options.RemotingOptions
		if system.options.StopTimeout > 0 && (remotingOptions.StopTimeout <= 0 || remotingOptions.StopTimeout > system.options.StopTimeout) {
			remotingOptions.StopTimeout = system.options.StopTimeout
		}
		system.options.Logger = system.options.Logger.With("addr", system.options.RemotingAdvertiseAddress)
		system.remotingAdmission = remoting.NewOutboundAdmission(remotingOptions, system.remotingEnvelopHandler, func(envelop vivid.Envelop, err error) {
			var address string
			if receiver := envelop.Receiver(); receiver != nil {
				address = receiver.GetAddress()
			}
			system.EventStream().Publish(system, ves.RemotingEnvelopSendFailedEvent{
				TargetAddress: address,
				Error:         err,
			})
		})
		ref, err := system.ActorOf(remoting.New(
			c.availableSignal,
			system.options.RemotingBindAddress,
			system.options.RemotingAdvertiseAddress,
			system.codec,
			system.remotingEnvelopHandler,
			remotingOptions,
			system.remotingAdmission,
		), vivid.WithActorName(remotingActorName))
		if err != nil {
			return err
		}
		system.remotingRef = ref
		system.remotingMailbox = remoting.NewMailbox(system.options.Context, system, ref, system, system.remotingAdmission)
		system.tell(true, system.Ref(), &guard.RegisterStopPriority{
			ActorRef: ref,
			Priority: remotingStopPriority,
		})
		return nil
	})
}

func (c *systemChains) validateVirtualActors(system *System) chain.Chain {
	return chain.ChainFN(func() error {
		options := system.virtualCoordinator.options
		if options.ActivationConcurrency < 0 || options.PendingMessageLimit < 0 {
			return vivid.ErrorIllegalArgument.WithMessage("virtual activation concurrency and pending message limit must not be negative")
		}
		return nil
	})
}

func (c *systemChains) initializeCluster(system *System) chain.Chain {
	return chain.ChainFN(func() (err error) {
		if system.options.ClusterOptions == nil {
			return nil
		}

		if system.remotingRef == nil {
			return vivid.ErrorGossipRequires.WithMessage("remoting is not initialized")
		}

		// 创建 gossip actor
		gossip := gossip.New(system.options.ClusterOptions)
		gossipRef, err := system.ActorOf(gossip)
		if err != nil {
			return err
		}
		system.statusLock.Lock()
		system.gossipRef = gossipRef
		system.clusterReadiness = gossip.Readiness()
		system.statusLock.Unlock()

		// 注册停止优先级
		system.tell(true, system.Ref(), &guard.RegisterStopPriority{
			ActorRef: gossipRef,
			Priority: gossipStopPriority,
		})
		return nil
	})
}
