package serialization

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageEncoderFN_Encode(t *testing.T) {
	called := false
	var lastWriter *Writer
	var lastMessage any
	fn := MessageEncoderFN(func(w *Writer, m any) error {
		called = true
		lastWriter = w
		lastMessage = m
		w.Write(uint32(123))
		return nil
	})

	w := GetWriter(nil)
	defer PutWriter(w)
	err := fn.Encode(w, 42)
	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, w, lastWriter)
	assert.Equal(t, 42, lastMessage)
	assert.Equal(t, uint32(123), binaryBigEndianUint32(w.Bytes()[0:4]))
}

func TestMessageDecoderFN_Decode(t *testing.T) {
	called := false
	var lastReader *Reader
	var lastMessage any
	fn := MessageDecoderFN(func(r *Reader, m any) error {
		called = true
		lastReader = r
		lastMessage = m
		return r.Read(m)
	})

	w := GetWriter(nil)
	defer PutWriter(w)
	w.Write(uint32(456))
	data := make([]byte, len(w.Bytes()))
	copy(data, w.Bytes())

	r := GetReader(nil, data)
	defer PutReader(r)
	var v uint32
	err := fn.Decode(r, &v)
	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, r, lastReader)
	assert.Equal(t, &v, lastMessage)
	assert.Equal(t, uint32(456), v)
}

func TestMessageCodec_Interface(t *testing.T) {
	var (
		_ MessageEncoder = MessageEncoderFN(nil)
		_ MessageDecoder = MessageDecoderFN(nil)
	)
	type both struct {
		MessageEncoderFN
		MessageDecoderFN
	}
	var _ MessageCodec = (*both)(nil)
}

// binaryBigEndianUint32 helper for test (avoid importing encoding/binary in test for one line).
func binaryBigEndianUint32(b []byte) uint32 {
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}
