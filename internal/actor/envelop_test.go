package actor_test

import (
	"testing"

	"github.com/kercylan98/vivid/internal/actor"
	"github.com/kercylan98/vivid/internal/mailbox"
	"github.com/kercylan98/vivid/pkg/vividkit"
	"github.com/stretchr/testify/assert"
)

func TestReplacedEnvelop(t *testing.T) {
	ref, err := vividkit.NewActorRef("localhost", "/test")
	assert.Nil(t, err)
	assert.NotNil(t, ref)
	envelop := mailbox.NewEnvelop(true, ref, ref, 1)
	replace := actor.ExportNewReplaceEnvelop(envelop, 1)
	assert.NotNil(t, replace)
	assert.Equal(t, true, replace.Sender().Equals(ref))
	assert.Equal(t, true, replace.Receiver().Equals(ref))
	assert.Equal(t, true, replace.System())
	assert.Equal(t, 1, replace.Message())
}
