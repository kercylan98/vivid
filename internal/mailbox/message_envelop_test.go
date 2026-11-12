package mailbox_test

import (
	"runtime"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/actor"
	"github.com/kercylan98/vivid/internal/mailbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type messageEnvelopHandler func(envelop vivid.Envelop)

func (fn messageEnvelopHandler) HandleEnvelop(envelop vivid.Envelop) {
	fn(envelop)
}

func TestEnqueueMessage_RetainedEnvelop(t *testing.T) {
	const count = 512
	sender, err := actor.NewRef("localhost", "/sender")
	require.NoError(t, err)
	receiver, err := actor.NewRef("localhost", "/receiver")
	require.NoError(t, err)
	received := make(chan vivid.Envelop, count)
	m := mailbox.NewUnboundedMailbox(4, messageEnvelopHandler(func(envelop vivid.Envelop) {
		received <- envelop
	}))
	var messages [count]int
	for i := range messages {
		messages[i] = i
		if i%17 == 0 {
			mailbox.EnqueueMessage[vivid.ActorRef](m, sender, receiver, &messages[i])
		} else {
			mailbox.EnqueueMessage(m, sender, receiver, &messages[i])
		}
	}

	envelops := make([]vivid.Envelop, count)
	timeout := time.After(3 * time.Second)
	for i := range envelops {
		select {
		case envelops[i] = <-received:
		case <-timeout:
			t.Fatal("receive envelop timeout")
		}
	}
	runtime.GC()
	for i, envelop := range envelops {
		assert.Same(t, sender, envelop.Sender())
		assert.Same(t, receiver, envelop.Receiver())
		assert.Same(t, &messages[i], envelop.Message())
		assert.Equal(t, i, *envelop.Message().(*int))
		assert.False(t, envelop.System())
	}
}
