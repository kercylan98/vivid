package actor

import (
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/future"
	"github.com/kercylan98/vivid/internal/gossip"
)

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
