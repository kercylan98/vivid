package actor_test

import (
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystem_ClusterShutdownSampling(t *testing.T) {
	for _, forced := range []bool{false, true} {
		name, interval, budget := "sampled", 20*time.Millisecond, 2*time.Second
		if forced {
			name, interval, budget = "budget", time.Hour, 80*time.Millisecond
		}
		t.Run(name, func(t *testing.T) {
			system := pendingClusterSystem(t, clusterTestAddress(t),
				vivid.WithActorSystemClusterGossipInterval(interval),
				vivid.WithActorSystemClusterGracefulShutdownTimeout(budget),
			)
			_, err := system.ClusterReady(time.Second).Result()
			require.NoError(t, err)
			started := time.Now()
			require.NoError(t, system.Stop(3*time.Second))
			elapsed := time.Since(started)
			if forced {
				assert.GreaterOrEqual(t, elapsed, budget, "周期长于退出预算时仍由原预算结束等待")
			} else {
				assert.GreaterOrEqual(t, elapsed, 9*interval, "三个退出阶段分别完成周期采样")
				assert.Less(t, elapsed, budget, "周期采样应主动完成，不能仅等待强制退出")
			}
		})
	}
}
