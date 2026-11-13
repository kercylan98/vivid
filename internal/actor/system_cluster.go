package actor

import (
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/future"
	"github.com/kercylan98/vivid/internal/gossip"
)

func (s *System) ClusterReady(timeout ...time.Duration) vivid.Future[vivid.ClusterJoinResult] {
	s.statusLock.Lock()
	defer s.statusLock.Unlock()
	switch s.status {
	case ready:
		return future.NewFutureFail[vivid.ClusterJoinResult](vivid.ErrorActorSystemNotStarted)
	case stop:
		return future.NewFutureFail[vivid.ClusterJoinResult](vivid.ErrorActorSystemStopped)
	}
	if s.options.ClusterOptions == nil {
		return future.NewFutureFail[vivid.ClusterJoinResult](vivid.ErrorClusterNotEnabled)
	}
	if s.clusterReadiness == nil {
		return future.NewFutureFail[vivid.ClusterJoinResult](vivid.ErrorGossipRequires.WithMessage("cluster is not initialized"))
	}
	waitTimeout := s.options.DefaultAskTimeout
	if len(timeout) > 0 {
		waitTimeout = timeout[0]
	}
	return s.clusterReadiness.Wait(s, waitTimeout)
}

func (s *System) ClusterSnapshot(timeout ...time.Duration) vivid.Future[vivid.ClusterSnapshot] {
	s.statusLock.Lock()
	defer s.statusLock.Unlock()
	switch s.status {
	case ready:
		return future.NewFutureFail[vivid.ClusterSnapshot](vivid.ErrorActorSystemNotStarted)
	case stop:
		return future.NewFutureFail[vivid.ClusterSnapshot](vivid.ErrorActorSystemStopped)
	}
	if s.options.ClusterOptions == nil {
		return future.NewFutureFail[vivid.ClusterSnapshot](vivid.ErrorClusterNotEnabled)
	}
	if s.gossipRef == nil {
		return future.NewFutureFail[vivid.ClusterSnapshot](vivid.ErrorGossipRequires.WithMessage("cluster is not initialized"))
	}
	return vivid.NewTypedFuture[vivid.Message, vivid.ClusterSnapshot](s.Ask(s.gossipRef, gossip.SnapshotRequest{}, timeout...))
}
