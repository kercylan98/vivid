package actor

import (
	"testing"

	"github.com/kercylan98/vivid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type tellMailbox struct {
	envelop vivid.Envelop
}

func (m *tellMailbox) Enqueue(envelop vivid.Envelop) {
	m.envelop = envelop
}

func (m *tellMailbox) Pause() {}

func (m *tellMailbox) Resume() {}

func (m *tellMailbox) IsPaused() bool {
	return false
}

func TestTellMessage_CustomMailbox(t *testing.T) {
	system := NewSystem()
	require.NoError(t, system.Start())
	defer func() { assert.NoError(t, system.Stop()) }()
	ref, err := NewRef(system.Ref().GetAddress(), "/custom")
	require.NoError(t, err)
	m := new(tellMailbox)
	ref.cacheMailbox(m)
	message := new(int)
	system.Tell(ref, message)
	require.NotNil(t, m.envelop)
	assert.Same(t, message, m.envelop.Message())
	assert.Same(t, system.Ref(), m.envelop.Sender())
	assert.Same(t, ref, m.envelop.Receiver())
	assert.False(t, m.envelop.System())

	system.TellWithSender(nil, ref, message)
	assert.Nil(t, m.envelop.Sender())
	assert.Same(t, message, m.envelop.Message())

	ctx := &Context{ref: ref, mailbox: m}
	ctx.TellSelf(message)
	assert.Same(t, ref, m.envelop.Sender())
	assert.Same(t, ref, m.envelop.Receiver())
	assert.Same(t, message, m.envelop.Message())
}
