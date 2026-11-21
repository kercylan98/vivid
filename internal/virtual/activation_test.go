package virtual_test

import (
	"testing"
	"time"

	"github.com/kercylan98/vivid/internal/serialization"
	"github.com/kercylan98/vivid/internal/virtual"
	"github.com/stretchr/testify/require"
)

func activationRequest() *virtual.ActivationRequest {
	return &virtual.ActivationRequest{Group: [32]byte{1}, Owner: [16]byte{2}, Kind: "worker", Name: "one", Timeout: time.Second}
}

func TestActivation_CodecAndTruncation(t *testing.T) {
	request := activationRequest()
	for _, generation := range []uint64{0, 7} {
		request.Generation = generation
		w := serialization.NewWriter()
		require.NoError(t, request.Encode(w, request))
		var decoded virtual.ActivationRequest
		require.NoError(t, decoded.Decode(serialization.NewReader(w.Bytes()), &decoded))
		require.Equal(t, *request, decoded)
		for end := range len(w.Bytes()) {
			old := *request
			require.Error(t, old.Decode(serialization.NewReader(w.Bytes()[:end]), &old))
			require.Equal(t, *request, old)
		}
		data := append([]byte(nil), w.Bytes()...)
		data[0] = 2
		require.ErrorIs(t, decoded.Decode(serialization.NewReader(data), &decoded), virtual.ErrInvalidActivation)
	}
	for status := virtual.ActivationReady; status <= virtual.ActivationUnknown; status++ {
		result := &virtual.ActivationResult{Group: request.Group, Owner: request.Owner, Kind: request.Kind, Name: request.Name, Generation: 7, Status: status}
		if status == virtual.ActivationReady {
			result.CurrentGeneration = 7
		}
		w := serialization.NewWriter()
		require.NoError(t, result.Encode(w, result))
		var decoded virtual.ActivationResult
		require.NoError(t, decoded.Decode(serialization.NewReader(w.Bytes()), &decoded))
		require.Equal(t, *result, decoded)
		for end := range len(w.Bytes()) {
			old := *result
			require.Error(t, old.Decode(serialization.NewReader(w.Bytes()[:end]), &old))
			require.Equal(t, *result, old)
		}
	}
}

func TestActivation_InvalidAndBudget(t *testing.T) {
	for _, change := range []func(*virtual.ActivationRequest){
		func(m *virtual.ActivationRequest) { m.Owner = [16]byte{} },
		func(m *virtual.ActivationRequest) { m.Group = [32]byte{} },
		func(m *virtual.ActivationRequest) { m.Timeout = 0 },
		func(m *virtual.ActivationRequest) { m.Kind = "\xff" },
		func(m *virtual.ActivationRequest) { m.Name = "" },
	} {
		m := activationRequest()
		change(m)
		require.False(t, m.Valid())
		require.ErrorIs(t, m.Encode(serialization.NewWriter(), m), virtual.ErrInvalidActivation)
	}
	m := activationRequest()
	result := &virtual.ActivationResult{Group: m.Group, Owner: m.Owner, Kind: m.Kind, Name: m.Name, Generation: 3, Status: virtual.ActivationReady}
	require.False(t, result.Valid())
	result.CurrentGeneration = 4
	require.False(t, result.Valid())
	result.Generation = 0
	require.True(t, result.Valid())
	result.Status = virtual.ActivationUnknown
	require.False(t, result.Valid(), "未知结果不能伪造已知代次")
	w := serialization.NewWriter()
	require.NoError(t, m.Encode(w, m))
	reader := serialization.NewReader(w.Bytes(), serialization.DecodeLimits{MaxAllocationBytes: 8})
	var decoded virtual.ActivationRequest
	require.ErrorIs(t, decoded.Decode(reader, &decoded), serialization.ErrDecodeLimit)
}

func FuzzActivationDecode(f *testing.F) {
	w := serialization.NewWriter()
	m := activationRequest()
	require.NoError(f, m.Encode(w, m))
	f.Add(w.Bytes())
	f.Fuzz(func(t *testing.T, data []byte) {
		limits := serialization.DecodeLimits{MaxAllocationBytes: 64 << 10, MaxDepth: 8}
		var request virtual.ActivationRequest
		if request.Decode(serialization.NewReader(data, limits), &request) == nil {
			require.True(t, request.Valid())
		}
		var result virtual.ActivationResult
		if result.Decode(serialization.NewReader(data, limits), &result) == nil {
			require.True(t, result.Valid())
		}
	})
}
