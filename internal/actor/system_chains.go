package actor

import (
	"net"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/chain"
	"github.com/kercylan98/vivid/internal/guard"
	metricsActor "github.com/kercylan98/vivid/internal/metrics"
	"github.com/kercylan98/vivid/internal/remoting"
	"github.com/kercylan98/vivid/pkg/metrics"
)

var systemChains = &_systemChains{}

type _systemChains struct{}

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
		metricsActor := metricsActor.NewActor(system.options.EnableMetricsUpdatedNotify)
		if _, err := system.ActorOf(metricsActor, vivid.WithActorName("@metrics")); err != nil {
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

		var remotingServerActorOptions = remoting.ServerActorOptions{}
		if system.testSystem != nil {
			remotingServerActorOptions.ListenerCreatedHandler = func(listener net.Listener) {
				system.testSystem.onBindRemotingListener(listener)
			}
		}

		system.remotingServer = remoting.NewServerActor(system.options.RemotingBindAddress, system.options.RemotingAdvertiseAddress, system.options.RemotingCodec, system, remotingServerActorOptions)
		_, err = system.ActorOf(system.remotingServer, vivid.WithActorName("@remoting"))
		return err
	})
}
