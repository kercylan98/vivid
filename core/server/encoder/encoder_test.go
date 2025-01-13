package encoder_test

import (
	"bytes"
	"github.com/kercylan98/vivid/core"
	"github.com/kercylan98/vivid/core/envelope"
	"github.com/kercylan98/vivid/core/id"
	"github.com/kercylan98/vivid/core/server/encoder"
	"testing"
)

func init() {
	core.GetMessageRegister().Register(new(TestMessage))
}

type TestMessage struct {
	Content string
}

func TestEncoder_Encode(t *testing.T) {

	var buf bytes.Buffer
	codec := encoder.Builder().Build(&buf)

	if err := codec.Encode(envelope.Builder().Build(
		id.Builder().Build("localhost", "/"),
		id.Builder().Build("localhost", "/"),
		&TestMessage{Content: "Hello"},
		core.UserMessage,
	)); err != nil {
		t.Fatal("Encode error:", err)
	}

	t.Log("Encode success:", buf.Len())
}
