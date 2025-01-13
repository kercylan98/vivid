package decoder_test

import (
	"bytes"
	"encoding/json"
	"github.com/kercylan98/vivid/pkg/vivid"
	"github.com/kercylan98/vivid/src/envelope"
	"github.com/kercylan98/vivid/src/process/id"
	"github.com/kercylan98/vivid/src/transport/server/decoder"
	"github.com/kercylan98/vivid/src/transport/server/encoder"
	"testing"
)

func init() {
	vivid.GetMessageRegister().Register(new(TestMessage))
}

type TestMessage struct {
	Content string
}

func TestDecoder_Decode(t *testing.T) {
	// Encode
	var buf bytes.Buffer
	e := encoder.Builder().Build(&buf)

	if err := e.Encode(envelope.Builder().Build(
		id.Builder().Build("localhost", "/"),
		id.Builder().Build("localhost", "/"),
		&TestMessage{Content: "Hello"},
		vivid.UserMessage,
	)); err != nil {
		t.Fatal("Encode error:", err)
	}

	// Decode

	d := decoder.Builder().Build(&buf, vivid.FnEnvelopeProvider(func() vivid.Envelope {
		return envelope.Builder().EmptyOf()
	}))
	m, err := d.Decode()
	if err != nil {
		t.Fatal("Decode error:", err)
	}

	jsonBytes, err := json.Marshal(m)
	if err != nil {
		t.Fatal("Marshal error:", err)
	}

	t.Log("Decode success:", string(jsonBytes))
}
