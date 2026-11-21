package actor

import (
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/remoting"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/require"
)

type probeRequest struct {
	system   bool
	sender   string
	receiver string
	message  any
}

type probeEnvelopHandler struct {
	remoting.NetworkEnvelopHandler
	requests chan probeRequest
}

func (h *probeEnvelopHandler) HandleRemotingEnvelop(system bool, sender, receiver string, message any) error {
	if heartbeat, ok := message.(*vivid.Heartbeat); ok && heartbeat.Ref == nil {
		h.requests <- probeRequest{system, sender, receiver, message}
		return nil
	}
	return h.NetworkEnvelopHandler.HandleRemotingEnvelop(system, sender, receiver, message)
}

func (h *probeEnvelopHandler) reply(t *testing.T, request probeRequest) {
	t.Helper()
	require.NoError(t, h.NetworkEnvelopHandler.HandleRemotingEnvelop(request.system, request.sender, request.receiver, request.message))
}

func TestSystem_ProbeRemoteTimeout(t *testing.T) {
	for _, timeout := range []struct {
		name   string
		values []time.Duration
		budget time.Duration
	}{
		{name: "default", budget: 200 * time.Millisecond},
		{name: "explicit", values: []time.Duration{20 * time.Millisecond}, budget: 20 * time.Millisecond},
		{name: "first value", values: []time.Duration{20 * time.Millisecond, time.Second}, budget: 20 * time.Millisecond},
	} {
		t.Run(timeout.name, func(t *testing.T) {
			client, ref, handler := newProbeTimeoutSystems(t, 200*time.Millisecond)
			probe := client.Probe(ref, timeout.values...)
			request := receiveRetryResult(t, handler.requests)
			heartbeat, err := probe.Result()
			require.Nil(t, heartbeat)
			require.ErrorIs(t, err, vivid.ErrorFutureTimeout)
			require.Contains(t, err.Error(), timeout.budget.String(), "超时应来自调用方预算或默认值")
			// 放行真实 TCP 上迟到的应答，再完成新探测，验证连接与后续请求仍可复用。
			handler.reply(t, request)
			next := client.Probe(ref, time.Second)
			handler.reply(t, receiveRetryResult(t, handler.requests))
			result, err := next.Result()
			require.NoError(t, err)
			require.True(t, result.Available)
			require.True(t, ref.Equals(result.Ref))
			heartbeat, err = probe.Result()
			require.Nil(t, heartbeat)
			require.ErrorIs(t, err, vivid.ErrorFutureTimeout, "迟到应答不能覆盖已发布的超时结果")
		})
	}
}

func TestSystem_ProbeRemoteTimeoutOverride(t *testing.T) {
	for _, timeout := range []struct {
		name  string
		value time.Duration
	}{
		{name: "longer", value: time.Second},
		{name: "zero", value: 0},
		{name: "negative", value: -time.Second},
	} {
		t.Run(timeout.name, func(t *testing.T) {
			client, ref, handler := newProbeTimeoutSystems(t, 40*time.Millisecond)
			probe := client.Probe(ref, timeout.value)
			defer probe.Close(nil)
			request := receiveRetryResult(t, handler.requests)
			// 等另一请求实际耗尽默认预算，再放行显式配置的探测，不以固定睡眠猜测超时。
			witness := client.Probe(ref)
			witnessRequest := receiveRetryResult(t, handler.requests)
			require.ErrorIs(t, witness.Wait(), vivid.ErrorFutureTimeout)
			handler.reply(t, request)
			result, err := probe.Result()
			require.NoError(t, err, "显式较长或非正预算不能被默认值提前结束")
			require.True(t, result.Available)
			handler.reply(t, witnessRequest)
		})
	}
}

func newProbeTimeoutSystems(t *testing.T, defaultTimeout time.Duration) (*System, vivid.ActorRef, *probeEnvelopHandler) {
	t.Helper()
	newSystem := func() *System {
		system := NewSystem(
			vivid.WithActorSystemRemoting(admissionSystemAddress(t)),
			vivid.WithActorSystemDefaultAskTimeout(defaultTimeout),
			vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
		)
		t.Cleanup(func() { require.NoError(t, system.Stop(time.Second)) })
		return system
	}
	server, client := newSystem(), newSystem()
	handler := &probeEnvelopHandler{NetworkEnvelopHandler: server.remotingEnvelopHandler, requests: make(chan probeRequest, 4)}
	server.remotingEnvelopHandler = handler
	require.NoError(t, server.Start())
	require.NoError(t, client.Start())
	ref, err := server.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {}))
	require.NoError(t, err)
	return client, ref, handler
}
