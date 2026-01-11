package messages

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type TestMessage struct {
	Bytes   []byte
	String  string
	Int64   int64
	Uint64  uint64
	Float64 float64
	Int32   int32
	Uint32  uint32
	Float32 float32
	Int16   int16
	Uint16  uint16
	Int8    int8
	Uint8   uint8
	Bool    bool
}

func TestMessageReaderAndWriter(t *testing.T) {

	message := TestMessage{
		Int8:    1,
		Int16:   2,
		Int32:   3,
		Int64:   4,
		Uint8:   5,
		Uint16:  6,
		Uint32:  7,
		Uint64:  8,
		Float32: 9,
		Float64: 10,
		Bool:    true,
		String:  "hello",
		Bytes:   []byte("world"),
	}

	writer := NewWriter()
	assert.NoError(t, writer.WriteFrom(
		int8(1), int16(2), int32(3), int64(4),
		uint8(5), uint16(6), uint32(7), uint64(8),
		float32(9), float64(10),
		bool(true),
		string("hello"),
		[]byte("world"),
		[]TestMessage{message, message, message},
		[]byte{1, 2, 3, 4, 5, 6, 7, 8},
		message,
	))

	assert.Empty(t, writer.Err(), "writer should not return error")

	reader := NewReader(writer.Bytes())

	var i8 int8
	var i16 int16
	var i32 int32
	var i64 int64
	var ui8 uint8
	var ui16 uint16
	var ui32 uint32
	var ui64 uint64
	var f32 float32
	var f64 float64
	var b bool
	var strHello string
	var bytesWorld []byte
	var testMessages []TestMessage
	var bytes2 []byte
	var testMessage TestMessage

	err := reader.ReadInto(
		&i8, &i16, &i32, &i64,
		&ui8, &ui16, &ui32, &ui64,
		&f32, &f64, &b, &strHello, &bytesWorld, &testMessages, &bytes2, &testMessage)

	assert.Empty(t, err, "reader should not return error")

	assert.Equal(t, i8, int8(1), "i8 should be 1")
	assert.Equal(t, i16, int16(2), "i16 should be 2")
	assert.Equal(t, i32, int32(3), "i32 should be 3")
	assert.Equal(t, i64, int64(4), "i64 should be 4")
	assert.Equal(t, ui8, uint8(5), "ui8 should be 5")
	assert.Equal(t, ui16, uint16(6), "ui16 should be 6")
	assert.Equal(t, ui32, uint32(7), "ui32 should be 7")
	assert.Equal(t, ui64, uint64(8), "ui64 should be 8")
	assert.Equal(t, f32, float32(9), "f32 should be 9")
	assert.Equal(t, f64, float64(10), "f64 should be 10")
	assert.Equal(t, b, true, "b should be true")
	assert.Equal(t, strHello, "hello", "strHello should be hello")
	assert.Equal(t, bytesWorld, []byte("world"), "bytesWorld should be world")
	assert.Equal(t, testMessages, []TestMessage{message, message, message}, "testMessages should be [message, message, message]")
	assert.Equal(t, bytes2, []byte{1, 2, 3, 4, 5, 6, 7, 8}, "bytes2 should be [1, 2, 3, 4, 5, 6, 7, 8]")
	assert.Equal(t, testMessage, message, "testMessage should be message")
	assert.Equal(t, reader.Pos(), len(writer.Bytes()), "reader pos should be equal to writer length")
	assert.Equal(t, reader.RemainingSize(), 0, "reader remaining size should be 0")
	assert.Equal(t, reader.Remaining(), []byte{}, "reader remaining should be empty")
}

func BenchmarkReader_WriteAndRead(b *testing.B) {
	message := TestMessage{
		Int8:    1,
		Int16:   2,
		Int32:   3,
		Int64:   4,
		Uint8:   5,
		Uint16:  6,
		Uint32:  7,
		Uint64:  8,
		Float32: 9,
		Float64: 10,
		Bool:    true,
		String:  "hello",
		Bytes:   []byte("world"),
	}

	for i := 0; i < b.N; i++ {
		writer := NewWriter()
		assert.NoError(b, writer.WriteFrom(
			int8(1), int16(2), int32(3), int64(4),
			uint8(5), uint16(6), uint32(7), uint64(8),
			float32(9), float64(10),
			bool(true),
			string("hello"),
			[]byte("world"),
			[]TestMessage{message, message, message},
			[]byte{1, 2, 3, 4, 5, 6, 7, 8},
			message,
		))
		assert.Empty(b, writer.Err(), "writer should not return error")
		reader := NewReader(writer.Bytes())
		var i8 int8
		var i16 int16
		var i32 int32
		var i64 int64
		var ui8 uint8
		var ui16 uint16
		var ui32 uint32
		var ui64 uint64
		var f32 float32
		var f64 float64
		var bb bool
		var strHello string
		var bytesWorld []byte
		var testMessages []TestMessage
		var bytes2 []byte
		var testMessage TestMessage
		err := reader.ReadInto(
			&i8, &i16, &i32, &i64,
			&ui8, &ui16, &ui32, &ui64,
			&f32, &f64, &bb, &strHello, &bytesWorld, &testMessages, &bytes2, &testMessage)
		assert.Empty(b, err, "reader should not return error")
	}
}
