package remoting

import (
	"math"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/stretchr/testify/assert"
)

func TestEndpointRetryPolicy_Options(t *testing.T) {
	options := vivid.NewActorSystemRemotingOptions(vivid.WithActorSystemRemotingReconnect(0, time.Second, 3*time.Second, 1, false))
	p := newEndpointRetryPolicy(&options)
	assert.Zero(t, p.limit, "0 表示首次连接失败后不重试")
	assert.Equal(t, time.Second, p.delayFor(0))
	assert.Equal(t, time.Second, p.delayFor(3), "因子 1 应使用固定间隔")
	options.ReconnectInitialDelay = 5 * time.Second
	p = newEndpointRetryPolicy(&options)
	assert.Equal(t, 3*time.Second, p.delayFor(0), "首次重试也受最大延迟约束")
	options = vivid.NewActorSystemRemotingOptions(vivid.WithActorSystemRemotingReconnect(2, time.Second, 8*time.Second, 2, true))
	p = newEndpointRetryPolicy(&options)
	var previous time.Duration
	var changed bool
	for i := 0; i < 64; i++ {
		delay := p.delayFor(2)
		assert.GreaterOrEqual(t, delay, 2*time.Second)
		assert.LessOrEqual(t, delay, 4*time.Second)
		changed = changed || i > 0 && delay != previous
		previous = delay
	}
	assert.True(t, changed, "启用抖动后应分散重连时间")
}

func TestEndpointRetryPolicy_Bounds(t *testing.T) {
	for _, jitter := range []bool{false, true} {
		for _, delay := range []time.Duration{1, 3, time.Second, time.Duration(math.MaxInt64)} {
			options := vivid.NewActorSystemRemotingOptions(vivid.WithActorSystemRemotingReconnect(2, delay, delay, 2, jitter))
			p := newEndpointRetryPolicy(&options)
			for _, retry := range []int{0, 1, 1024, math.MaxInt} {
				actual := p.delayFor(retry)
				assert.Greater(t, actual, time.Duration(0))
				assert.LessOrEqual(t, actual, delay)
				if !jitter {
					assert.Equal(t, delay, actual)
				}
			}
		}
	}
	options := vivid.NewActorSystemRemotingOptions(vivid.WithActorSystemRemotingReconnect(2, time.Second, 10*time.Second, 2, false))
	p := newEndpointRetryPolicy(&options)
	assert.Equal(t, 10*time.Second, p.delayFor(math.MaxInt))
	for _, factor := range []float64{0, 0.5, math.NaN(), math.Inf(1)} {
		options := vivid.NewActorSystemRemotingOptions(vivid.WithActorSystemRemotingReconnectFactor(factor))
		assert.Equal(t, 2.5, options.ReconnectFactor)
		options = vivid.NewActorSystemRemotingOptions(vivid.WithActorSystemRemotingReconnect(0, time.Second, time.Second, factor, false))
		assert.Equal(t, 2.5, options.ReconnectFactor)
		options.ReconnectFactor = factor
		assert.Equal(t, 2.5, newEndpointRetryPolicy(&options).backoff)
	}
}
