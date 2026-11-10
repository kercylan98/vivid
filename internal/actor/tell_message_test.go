package actor_test

import (
	"net"
	"runtime"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/actor"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func receiveTellResult[T any](t *testing.T, received <-chan T) T {
	t.Helper()
	select {
	case result := <-received:
		return result
	case <-time.After(3 * time.Second):
		t.Fatal("receive tell result timeout")
		var result T
		return result
	}
}

func TestTellMessage_Stash(t *testing.T) {
	type message struct {
		index int
	}
	type unstash struct{}
	const count = 256
	system := actor.NewSystem()
	require.NoError(t, system.Start())
	defer func() { assert.NoError(t, system.Stop()) }()
	received := make(chan int, count)
	senders := make(chan vivid.ActorRef, count)
	stashing := true
	ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch msg := ctx.Message().(type) {
		case *message:
			if stashing {
				ctx.Stash()
			} else {
				received <- msg.index
				senders <- ctx.Sender()
			}
		case unstash:
			stashing = false
			runtime.GC()
			ctx.Unstash(0)
		}
	}))
	require.NoError(t, err)
	for i := 0; i < count; i++ {
		system.Tell(ref, &message{index: i})
	}
	system.Tell(ref, unstash{})
	for i := 0; i < count; i++ {
		assert.Equal(t, i, receiveTellResult(t, received))
		assert.True(t, receiveTellResult(t, senders).Equals(system.Ref()))
	}
}

func TestTellMessage_CachedRefAcrossSystems(t *testing.T) {
	listener1, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener1.Close()
	listener2, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener2.Close()
	address1, address2 := listener1.Addr().String(), listener2.Addr().String()
	require.NoError(t, listener1.Close())
	require.NoError(t, listener2.Close())

	system1 := actor.NewSystem(vivid.WithActorSystemRemoting(address1))
	system2 := actor.NewSystem(vivid.WithActorSystemRemoting(address2))
	require.NoError(t, system1.Start())
	defer func() { assert.NoError(t, system1.Stop(3*time.Second)) }()
	require.NoError(t, system2.Start())
	defer func() { assert.NoError(t, system2.Stop(3*time.Second)) }()
	received := make(chan string, 3)
	ref, err := system1.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		if _, ok := ctx.Message().(vivid.ActorRef); ok {
			received <- ctx.Sender().GetAddress()
		}
	}))
	require.NoError(t, err)

	system1.Tell(ref, ref)
	assert.Equal(t, address1, receiveTellResult(t, received))
	system2.Tell(ref, ref)
	assert.Equal(t, address2, receiveTellResult(t, received))
	system1.Tell(ref, ref)
	assert.Equal(t, address1, receiveTellResult(t, received))
}

func TestTellMessage_DeathLetterRetainsEnvelop(t *testing.T) {
	const count = 128
	system := actor.NewSystem()
	require.NoError(t, system.Start())
	defer func() { assert.NoError(t, system.Stop()) }()
	received := make(chan vivid.Envelop, count)
	ready := make(chan struct{})
	_, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch msg := ctx.Message().(type) {
		case *vivid.OnLaunch:
			ctx.EventStream().Subscribe(ctx, ves.DeathLetterEvent{})
			close(ready)
		case ves.DeathLetterEvent:
			received <- msg.Envelope
		}
	}))
	require.NoError(t, err)
	receiveTellResult(t, ready)
	warmed := make(chan struct{})
	killed := make(chan struct{})
	ref, err := system.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		switch ctx.Message().(type) {
		case int:
			close(warmed)
		case *vivid.OnKilled:
			close(killed)
		}
	}))
	require.NoError(t, err)
	system.Tell(ref, -1)
	receiveTellResult(t, warmed)
	system.Kill(ref, false)
	receiveTellResult(t, killed)

	var messages [count]int
	for i := range messages {
		messages[i] = i
		system.Tell(ref, &messages[i])
	}
	envelops := make([]vivid.Envelop, count)
	for i := range envelops {
		envelops[i] = receiveTellResult(t, received)
	}
	runtime.GC()
	for i, envelop := range envelops {
		assert.Same(t, system.Ref(), envelop.Sender())
		assert.Same(t, ref, envelop.Receiver())
		assert.Same(t, &messages[i], envelop.Message())
		assert.Equal(t, i, *envelop.Message().(*int))
	}
}
