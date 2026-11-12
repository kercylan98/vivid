package vivid_test

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/bootstrap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type publicCodecPayload struct {
	ID    uint32
	Ratio float32
	Text  string
}

func encodePublicPayload(w *vivid.Writer, message any) error {
	value, ok := message.(*publicCodecPayload)
	if !ok {
		return fmt.Errorf("unexpected message %T", message)
	}
	w.WriteInteger(value.ID).WriteFloat(value.Ratio).WriteString(value.Text)
	return w.Err()
}

func decodePublicPayload(r *vivid.Reader, message any) error {
	value := message.(*publicCodecPayload)
	if err := r.ReadInteger(&value.ID); err != nil {
		return err
	}
	if err := r.ReadFloat(&value.Ratio); err != nil {
		return err
	}
	return r.ReadString(&value.Text)
}

type publicWireCodec struct{}

func (*publicWireCodec) Encode(w *vivid.Writer, message any) error {
	return encodePublicPayload(w, message)
}
func (*publicWireCodec) Decode(r *vivid.Reader) (any, error) {
	var value publicCodecPayload
	err := decodePublicPayload(r, &value)
	return &value, err
}

func TestCodec_PublicReaderWriter(t *testing.T) {
	want := publicCodecPayload{ID: 42, Ratio: 1.5, Text: "你好，Vivid"}
	w := vivid.NewWriter()
	require.NoError(t, encodePublicPayload(w, &want))
	r := vivid.NewReader(w.Bytes())
	var got publicCodecPayload
	require.NoError(t, decodePublicPayload(r, &got))
	assert.Equal(t, want, got)
}

func TestCodec_PublicDecodeLimits(t *testing.T) {
	limits := vivid.DecodeLimits{MaxAllocationBytes: 3}
	w := vivid.NewWriter().WriteString("1234")
	r := vivid.NewReader(w.Bytes(), limits)
	var text string
	require.ErrorIs(t, r.ReadString(&text), vivid.ErrDecodeLimit)
	options := vivid.NewActorSystemOptions(vivid.WithActorSystemRemotingOption(vivid.WithActorSystemRemotingDecodeLimits(limits)))
	assert.Equal(t, limits, options.RemotingOptions.DecodeLimits)
	assert.Equal(t, vivid.DefaultDecodeLimits(), vivid.NewActorSystemRemotingOptions().DecodeLimits)
}

func TestCodec_Remoting(t *testing.T) {
	for _, tc := range []struct {
		name   string
		option vivid.ActorSystemOption
	}{
		{name: "external", option: vivid.WithActorSystemCodec(&publicWireCodec{})},
		{name: "registered", option: vivid.WithActorSystemMessageRegister(vivid.MessageRegisterFN(func(registry vivid.MessageRegistry) {
			registry.RegisterMessageWithEncoderAndDecoder("PublicPayload", new(publicCodecPayload),
				vivid.MessageEncoderFN(encodePublicPayload), vivid.MessageDecoderFN(decodePublicPayload))
		}))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			newSystem := func() vivid.PrimaryActorSystem {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)
				address := listener.Addr().String()
				require.NoError(t, listener.Close())
				system := bootstrap.NewActorSystem(vivid.WithActorSystemRemoting(address), tc.option)
				require.NoError(t, system.Start())
				t.Cleanup(func() { assert.NoError(t, system.Stop(3*time.Second)) })
				return system
			}
			first, second := newSystem(), newSystem()
			received := make(chan publicCodecPayload, 1)
			ref, err := first.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
				if message, ok := ctx.Message().(*publicCodecPayload); ok {
					received <- *message
				}
			}))
			require.NoError(t, err)
			want := publicCodecPayload{ID: 42, Ratio: 1.5, Text: "跨节点消息"}
			second.Tell(ref, &want)
			select {
			case got := <-received:
				assert.Equal(t, want, got)
			case <-time.After(3 * time.Second):
				t.Fatal("receive codec message timeout")
			}
		})
	}
}
