package encoder_test

import (
	"bytes"
	"github.com/kercylan98/vivid/pkg/vivid"
	"github.com/kercylan98/vivid/src/envelope"
	"github.com/kercylan98/vivid/src/process/id"
	"github.com/kercylan98/vivid/src/transport/server/encoder"
	"testing"
)

func init() {
	vivid.GetMessageRegister().Register(new(TestMessage))
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
		vivid.UserMessage,
	)); err != nil {
		t.Fatal("Encode error:", err)
	}

	t.Log("Encode success:", buf.Len())
}
