package clusterstore

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommand_Codec(t *testing.T) {
	for _, command := range []Command{
		{}, {Operation: Put, Key: "singleton/订单", Value: []byte("node-a"), ExpectedRevision: ^uint64(0)},
		{Operation: Put, Key: "empty"}, {Operation: Delete, Key: "owner", ExpectedRevision: 5},
		{Operation: Put, Key: strings.Repeat("k", maxKeyBytes), Value: make([]byte, maxValueBytes)},
	} {
		data, err := EncodeCommand(command)
		require.NoError(t, err)
		decoded, err := DecodeCommand(data)
		require.NoError(t, err)
		require.Equal(t, command.Operation, decoded.Operation)
		require.Equal(t, command.Key, decoded.Key)
		require.Equal(t, command.ExpectedRevision, decoded.ExpectedRevision)
		require.True(t, bytes.Equal(command.Value, decoded.Value))
		again, err := EncodeCommand(decoded)
		require.NoError(t, err)
		require.Equal(t, data, again)
	}
	value := []byte("node-a")
	data, err := EncodeCommand(Command{Operation: Put, Key: "owner", Value: value})
	require.NoError(t, err)
	value[0] = 'x'
	decoded, err := DecodeCommand(data)
	require.NoError(t, err)
	var store Store
	_, err = store.Apply(1, decoded)
	require.NoError(t, err)
	clear(data)
	record, exists := store.Get("owner")
	require.True(t, exists)
	require.Equal(t, []byte("node-a"), record.Value, "日志释放后状态机仍持有独立副本")
}

func TestCommand_Invalid(t *testing.T) {
	data, err := EncodeCommand(Command{Operation: Put, Key: "owner", Value: []byte("node-a")})
	require.NoError(t, err)
	for size := 1; size < len(data); size++ {
		_, err := DecodeCommand(data[:size])
		require.ErrorIs(t, err, ErrInvalidCommand)
	}
	for _, mutate := range []func([]byte) []byte{
		func(data []byte) []byte { data[0] ^= 1; return data },
		func(data []byte) []byte { data[8] = 0; return data },
		func(data []byte) []byte { data[8] = 255; return data },
		func(data []byte) []byte { data[8] = byte(Delete); return data },
		func(data []byte) []byte { data[15] = 1; return data },
		func(data []byte) []byte { binary.LittleEndian.PutUint32(data[24:], ^uint32(0)); return data },
		func(data []byte) []byte { binary.LittleEndian.PutUint32(data[28:], ^uint32(0)); return data },
		func(data []byte) []byte { data[commandHeaderBytes] = 255; return data },
		func(data []byte) []byte { return append(data, 0) },
	} {
		_, err := DecodeCommand(mutate(bytes.Clone(data)))
		require.ErrorIs(t, err, ErrInvalidCommand)
	}
	for _, command := range []Command{
		{Operation: Put, Key: ""}, {Operation: Put, Key: strings.Repeat("k", maxKeyBytes+1)},
		{Operation: Put, Key: "key", Value: make([]byte, maxValueBytes+1)}, {Operation: Delete, Key: "key", Value: []byte("x")},
	} {
		_, err := EncodeCommand(command)
		require.ErrorIs(t, err, ErrInvalidCommand)
	}
}

func FuzzCommand_Decode(f *testing.F) {
	data, _ := EncodeCommand(Command{Operation: Put, Key: "owner", Value: []byte("node-a")})
	f.Add(data)
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 16<<10 {
			return
		}
		command, err := DecodeCommand(data)
		if err == nil {
			encoded, err := EncodeCommand(command)
			require.NoError(t, err)
			require.True(t, bytes.Equal(data, encoded))
		}
	})
}

func BenchmarkCommand_EncodeDecode(b *testing.B) {
	for _, size := range []int{64, 1 << 20} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			command := Command{Operation: Put, Key: "singleton/orders", Value: make([]byte, size)}
			b.ReportAllocs()
			for b.Loop() {
				data, err := EncodeCommand(command)
				if err != nil {
					b.Fatal(err)
				}
				if _, err := DecodeCommand(data); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
