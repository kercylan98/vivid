package serialization

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

type structFallbackDetail struct{ Value int16 }
type structFallbackPayload struct {
	ID     uint32
	Detail structFallbackDetail
	Values []uint16
}

func TestWriter_Write_StructFallback(t *testing.T) {
	message := structFallbackPayload{ID: 42, Detail: structFallbackDetail{Value: -2}, Values: []uint16{3, 4}}
	// 固定字节验证字段顺序和编码格式，避免读写两侧同时出错却能往返成功。
	want := []byte{0, 0, 0, 42, 255, 254, 0, 0, 0, 2, 0, 3, 0, 4}
	for _, value := range []any{message, &message} {
		w := GetWriter()
		w.Write(value)
		require.NoError(t, w.Err())
		assert.Equal(t, want, w.Bytes())
		PutWriter(w)
	}
}

func TestReader_Read_StructFallback(t *testing.T) {
	r := GetReader([]byte{0, 0, 0, 42, 255, 254, 0, 0, 0, 2, 0, 3, 0, 4})
	defer PutReader(r)
	var message structFallbackPayload
	require.NoError(t, r.Read(&message))
	assert.Equal(t, structFallbackPayload{ID: 42, Detail: structFallbackDetail{Value: -2}, Values: []uint16{3, 4}}, message)
}

func TestReader_Read_StructFallbackTruncated(t *testing.T) {
	r := GetReader([]byte{0, 0, 0, 42, 255})
	defer PutReader(r)
	var message structFallbackPayload
	err := r.Read(&message)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "EOF")
	assert.Equal(t, uint32(42), message.ID)
	assert.Equal(t, err, r.Read(&message))
}

type structFallbackMessage struct{ value uint32 }

func (m *structFallbackMessage) Encode(w *Writer, message any) error {
	w.WriteInteger(message.(*structFallbackMessage).value)
	return w.Err()
}
func (m *structFallbackMessage) Decode(r *Reader, message any) error {
	return r.ReadInteger(&message.(*structFallbackMessage).value)
}

func TestStructFallback_MessageCodecPriority(t *testing.T) {
	w := GetWriter()
	defer PutWriter(w)
	w.Write(&structFallbackMessage{value: 42})
	require.NoError(t, w.Err())
	assert.Equal(t, []byte{0, 0, 0, 42}, w.Bytes())
	r := GetReader([]byte{0, 0, 0, 42})
	defer PutReader(r)
	var message structFallbackMessage
	require.NoError(t, r.Read(&message))
	assert.Equal(t, uint32(42), message.value)
}
