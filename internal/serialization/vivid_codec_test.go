package serialization

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewVividCodec(t *testing.T) {
	c := NewVividCodec(nil)
	require.NotNil(t, c)
	assert.NotNil(t, c.fullName2desc)
	assert.NotNil(t, c.tof2desc)
	assert.Nil(t, c.externalCodec)

	c2 := NewVividCodec(mockCodecFunc)
	assert.NotNil(t, c2.externalCodec)
}

var mockCodecFunc = &mockCodec{}

type mockCodec struct{}

func (m *mockCodec) Encode(w *Writer, message any) error { return nil }
func (m *mockCodec) Decode(r *Reader) (any, error)       { return nil, nil }

func TestVividCodec_RegisterMessage_FindMessageDesc(t *testing.T) {
	c := NewVividCodec(nil)

	type MyMsg struct {
		V int32
	}
	var my MyMsg
	assert.NoError(t, c.RegisterMessageWithEncoderAndDecoder("test", "MyMsg", &my,
		MessageEncoderFN(func(w *Writer, m any) error { return nil }),
		MessageDecoderFN(func(r *Reader, m any) error { return nil }),
	))
	desc := c.FindMessageDesc("test", "MyMsg")
	require.NotNil(t, desc)
	assert.Equal(t, "test.MyMsg", desc.FullName())
	assert.Equal(t, "test", desc.Class())
	assert.Equal(t, "MyMsg", desc.Name())

	descByType := c.FindMessageDescByType(reflect.TypeOf(&my))
	assert.Equal(t, desc, descByType)

	descByFull := c.FindMessageDescByFullName("test.MyMsg")
	assert.Equal(t, desc, descByFull)

	assert.Nil(t, c.FindMessageDesc("other", "X"))
	assert.Nil(t, c.FindMessageDescByFullName("unknown.Foo"))
}

func TestVividCodec_RegisterMessage_DuplicatePanics(t *testing.T) {
	c := NewVividCodec(nil)
	type MyMsg struct{ V int32 }
	var my MyMsg

	assert.NoError(t, c.RegisterMessageWithEncoderAndDecoder("pkg", "MyMsg", &my,
		MessageEncoderFN(func(w *Writer, m any) error { return nil }),
		MessageDecoderFN(func(r *Reader, m any) error { return nil }),
	))
	assert.Error(t, c.RegisterMessageWithEncoderAndDecoder("pkg", "MyMsg", &my,
		MessageEncoderFN(func(w *Writer, m any) error { return nil }),
		MessageDecoderFN(func(r *Reader, m any) error { return nil }),
	))
}

func TestVividCodec_EncodeDecode_Roundtrip(t *testing.T) {
	type Payload struct {
		N int32
		S string
	}

	enc := MessageEncoderFN(func(w *Writer, m any) error {
		p := m.(*Payload)
		w.Write(p.N, p.S)
		return nil
	})
	dec := MessageDecoderFN(func(r *Reader, m any) error {
		p := m.(*Payload)
		return r.Read(&p.N, &p.S)
	})

	c := NewVividCodec(nil)
	var sample Payload
	assert.NoError(t, c.RegisterMessageWithEncoderAndDecoder("app", "Payload", &sample, enc, dec))

	w := GetWriter(c)
	defer PutWriter(w)
	msg := &Payload{N: 42, S: "hello"}
	err := c.Encode(w, msg)
	require.NoError(t, err)
	data := make([]byte, len(w.Bytes()))
	copy(data, w.Bytes())

	r := GetReader(c, data)
	defer PutReader(r)
	decoded, err := c.Decode(r)
	require.NoError(t, err)
	require.IsType(t, &Payload{}, decoded)
	assert.Equal(t, int32(42), decoded.(*Payload).N)
	assert.Equal(t, "hello", decoded.(*Payload).S)
}

func TestVividCodec_Encode_UnregisteredReturnsError(t *testing.T) {
	c := NewVividCodec(nil)
	w := GetWriter(c)
	defer PutWriter(w)

	type Unregistered struct{ X int32 }
	assert.Error(t, c.Encode(w, &Unregistered{X: 1}))
}

func TestVividCodec_Decode_UnregisteredFullnameReturnsError(t *testing.T) {
	c := NewVividCodec(nil)
	w := GetWriter(nil)
	defer PutWriter(w)
	w.Write("unknown.Foo", int32(1)) // fullname + some body
	data := make([]byte, len(w.Bytes()))
	copy(data, w.Bytes())

	r := GetReader(c, data)
	defer PutReader(r)
	_, err := c.Decode(r)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not registered")
}

func TestVividCodec_MessageDesc_Instantiate(t *testing.T) {
	type T struct{ V int32 }
	var x T
	c := NewVividCodec(nil)
	assert.NoError(t, c.RegisterMessageWithEncoderAndDecoder("p", "T", &x,
		MessageEncoderFN(func(w *Writer, m any) error { return nil }),
		MessageDecoderFN(func(r *Reader, m any) error { return nil }),
	))
	desc := c.FindMessageDescByType(reflect.TypeOf(&x))
	require.NotNil(t, desc)
	inst := desc.Instantiate()
	require.NotNil(t, inst)
	_, ok := inst.(*T)
	assert.True(t, ok)
}

func TestGenerateMessageDescFullname(t *testing.T) {
	// 通过 Register + Find 间接验证 fullname 格式
	c := NewVividCodec(nil)
	type X struct{ A int32 }
	var x X
	assert.NoError(t, c.RegisterMessageWithEncoderAndDecoder("pkg", "X", &x,
		MessageEncoderFN(func(w *Writer, m any) error { return nil }),
		MessageDecoderFN(func(r *Reader, m any) error { return nil }),
	))
	d := c.FindMessageDescByFullName("pkg.X")
	require.NotNil(t, d)
	assert.Equal(t, "pkg", d.Class())
	assert.Equal(t, "X", d.Name())
	assert.Equal(t, "pkg.X", d.FullName())
}
