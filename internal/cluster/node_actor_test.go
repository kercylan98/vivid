package cluster

import (
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testClusterOptions 返回用于单测的集群选项：短间隔与短超时，避免测试卡死或长时间等待。
// 集成测试因 import cycle 不能放在本包，可在外层（如 internal/actor 或 pkg）使用本函数构建选项。
// 可传入额外 ClusterOption 覆盖默认（如 NodeID、ClusterName、Seeds）。
func testClusterOptions(opts ...vivid.ClusterOption) *vivid.ClusterOptions {
	base := []vivid.ClusterOption{
		vivid.WithClusterDiscoveryInterval(50 * time.Millisecond),
		vivid.WithClusterFailureDetectionTimeout(200 * time.Millisecond),
		vivid.WithClusterJoinAskTimeout(time.Second),
		vivid.WithClusterGetViewAskTimeout(time.Second),
		vivid.WithClusterLeaveBroadcastDelay(20 * time.Millisecond),
		vivid.WithClusterLeaveBroadcastRounds(2),
	}
	return vivid.NewClusterOptions(append(base, opts...)...)
}

func TestTestClusterOptions_ShortIntervalsAndTimeouts(t *testing.T) {
	deadline := time.Now().Add(testCaseTimeout)
	if d, ok := t.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	t.Cleanup(func() {
		if time.Now().After(deadline) {
			t.Error("test exceeded timeout")
		}
	})

	opts := testClusterOptions(
		vivid.WithClusterNodeID("n1"),
		vivid.WithClusterName("test"),
	)
	require.NotNil(t, opts)
	assert.Equal(t, 50*time.Millisecond, opts.DiscoveryInterval)
	assert.Equal(t, 200*time.Millisecond, opts.FailureDetectionTimeout)
	assert.Equal(t, time.Second, opts.JoinAskTimeout)
	assert.Equal(t, time.Second, opts.GetViewAskTimeout)
	assert.Equal(t, 20*time.Millisecond, opts.LeaveBroadcastDelay)
	assert.Equal(t, 2, opts.LeaveBroadcastRounds)
}
