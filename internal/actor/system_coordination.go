package actor

import (
	"slices"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/chain"
	"github.com/kercylan98/vivid/internal/clusterstore"
	"github.com/kercylan98/vivid/internal/future"
	"github.com/kercylan98/vivid/internal/guard"
	"github.com/kercylan98/vivid/internal/utils"
)

func (s *System) validateCoordination() error {
	o := s.options.CoordinationOptions
	if err := o.Validate(); err != nil {
		return err
	}
	if s.options.RemotingBindAddress == "" || o.Members[o.NodeID] == "" {
		return vivid.ErrorIllegalArgument.WithMessage("coordination requires Remoting")
	}
	local, _ := utils.NormalizeRemoteAddress(o.Members[o.NodeID])
	if local != s.options.RemotingAdvertiseAddress {
		return vivid.ErrorIllegalArgument.WithMessage("coordination local member must match Remoting advertise address")
	}
	return nil
}

func (c *systemChains) initializeCoordination(s *System) chain.Chain {
	return chain.ChainFN(func() error {
		o := s.options.CoordinationOptions
		if o == nil {
			return nil
		}
		routes := make(map[uint64]vivid.ActorRef, len(o.Members))
		voters := make([]uint64, 0, len(o.Members))
		for id, address := range o.Members {
			ref, err := NewRef(address, "/"+coordinationActorName)
			if err != nil {
				return vivid.ErrorIllegalArgument.With(err)
			}
			routes[id] = ref
			voters = append(voters, id)
		}
		slices.Sort(voters)
		driver, err := clusterstore.NewRaftActor(clusterstore.RaftActorOptions{
			Path: o.Path, Config: clusterstore.RaftConfig{GroupID: o.GroupID, NodeID: o.NodeID, Voters: voters},
			Create: o.Create, Routes: routes, HeartbeatInterval: o.HeartbeatInterval, ElectionMin: o.ElectionMin, ElectionMax: o.ElectionMax,
		})
		if err != nil {
			return vivid.ErrorIllegalArgument.With(err)
		}
		ref, err := s.ActorOf(driver, vivid.WithActorName(coordinationActorName))
		if err != nil {
			return err
		}
		s.statusLock.Lock()
		s.coordinationRef = ref
		s.statusLock.Unlock()
		s.tell(true, s.Ref(), &guard.RegisterStopPriority{ActorRef: ref, Priority: coordinationStopPriority})
		// 本地 OnLaunch 完成 WAL 创建或恢复后才回应；不等待网络选主，也不套用业务 Ask 默认超时。
		return s.Ask(ref, clusterstore.CoordinationStatusRequest{}, 0).Wait()
	})
}

func (s *System) CoordinationStatus(timeout ...time.Duration) vivid.Future[vivid.CoordinationStatus] {
	s.statusLock.Lock()
	if s.status == ready {
		s.statusLock.Unlock()
		return future.NewFutureFail[vivid.CoordinationStatus](vivid.ErrorActorSystemNotStarted)
	}
	if s.status == stop {
		s.statusLock.Unlock()
		return future.NewFutureFail[vivid.CoordinationStatus](vivid.ErrorActorSystemStopped)
	}
	ref, enabled := s.coordinationRef, s.options.CoordinationOptions != nil
	s.statusLock.Unlock()
	if !enabled {
		return future.NewFutureFail[vivid.CoordinationStatus](vivid.ErrorCoordinationNotEnabled)
	}
	if ref == nil {
		return future.NewFutureFail[vivid.CoordinationStatus](vivid.ErrorCoordinationUnavailable)
	}
	return vivid.NewTypedFuture[vivid.Message, vivid.CoordinationStatus](s.Ask(ref, clusterstore.CoordinationStatusRequest{}, timeout...))
}
