package virtual_test

import (
	"strings"
	"testing"

	"github.com/kercylan98/vivid/internal/serialization"
	"github.com/kercylan98/vivid/internal/virtual"
	"github.com/stretchr/testify/require"
)

func TestDiscovery_CodecAndTruncation(t *testing.T) {
	request := &virtual.DiscoveryRequest{Group: [32]byte{1}, Kind: "worker"}
	w := serialization.NewWriter()
	require.NoError(t, request.Encode(w, request))
	var decoded virtual.DiscoveryRequest
	require.NoError(t, decoded.Decode(serialization.NewReader(w.Bytes()), &decoded))
	require.Equal(t, *request, decoded)
	for end := range len(w.Bytes()) {
		old := *request
		require.Error(t, old.Decode(serialization.NewReader(w.Bytes()[:end]), &old))
		require.Equal(t, *request, old)
	}
	data := append([]byte(nil), w.Bytes()...)
	data[0] = 2
	require.ErrorIs(t, decoded.Decode(serialization.NewReader(data), &decoded), virtual.ErrInvalidDiscovery)
	for status := virtual.DiscoveryAvailable; status <= virtual.DiscoveryInvalid; status++ {
		result := &virtual.DiscoveryResult{Group: request.Group, Kind: request.Kind, Status: status}
		if status == virtual.DiscoveryAvailable {
			result.Owner = [16]byte{2}
		}
		w := serialization.NewWriter()
		require.NoError(t, result.Encode(w, result))
		var decoded virtual.DiscoveryResult
		require.NoError(t, decoded.Decode(serialization.NewReader(w.Bytes()), &decoded))
		require.Equal(t, *result, decoded)
		for end := range len(w.Bytes()) {
			old := *result
			require.Error(t, old.Decode(serialization.NewReader(w.Bytes()[:end]), &old))
			require.Equal(t, *result, old)
		}
		data := append([]byte(nil), w.Bytes()...)
		data[0] = 2
		require.ErrorIs(t, decoded.Decode(serialization.NewReader(data), &decoded), virtual.ErrInvalidDiscovery)
	}
}

func TestDiscovery_InvalidAndBudget(t *testing.T) {
	for _, m := range []*virtual.DiscoveryRequest{
		{}, {Group: [32]byte{1}}, {Group: [32]byte{1}, Kind: "\xff"}, {Group: [32]byte{1}, Kind: strings.Repeat("a", 1025)},
	} {
		require.False(t, m.Valid())
		require.ErrorIs(t, m.Encode(serialization.NewWriter(), m), virtual.ErrInvalidDiscovery)
	}
	for _, m := range []*virtual.DiscoveryResult{
		{}, {Group: [32]byte{1}, Kind: "worker", Status: virtual.DiscoveryAvailable},
		{Group: [32]byte{1}, Kind: "worker", Status: virtual.DiscoveryStopped, Owner: [16]byte{1}},
		{Group: [32]byte{1}, Kind: "worker", Status: 5},
	} {
		require.False(t, m.Valid())
		require.ErrorIs(t, m.Encode(serialization.NewWriter(), m), virtual.ErrInvalidDiscovery)
	}
	request := &virtual.DiscoveryRequest{Group: [32]byte{1}, Kind: "worker"}
	w := serialization.NewWriter()
	require.NoError(t, request.Encode(w, request))
	old := *request
	require.ErrorIs(t, old.Decode(serialization.NewReader(w.Bytes(), serialization.DecodeLimits{MaxAllocationBytes: 1}), &old), serialization.ErrDecodeLimit)
	require.Equal(t, *request, old)
}

func FuzzDiscoveryDecode(f *testing.F) {
	request := &virtual.DiscoveryRequest{Group: [32]byte{1}, Kind: "worker"}
	w := serialization.NewWriter()
	require.NoError(f, request.Encode(w, request))
	f.Add(w.Bytes())
	result := &virtual.DiscoveryResult{Group: request.Group, Kind: request.Kind, Status: virtual.DiscoveryAvailable, Owner: [16]byte{2}}
	w = serialization.NewWriter()
	require.NoError(f, result.Encode(w, result))
	f.Add(w.Bytes())
	f.Fuzz(func(t *testing.T, data []byte) {
		limits := serialization.DecodeLimits{MaxAllocationBytes: 64 << 10, MaxDepth: 8}
		var request virtual.DiscoveryRequest
		if request.Decode(serialization.NewReader(data, limits), &request) == nil {
			require.True(t, request.Valid())
		}
		var result virtual.DiscoveryResult
		if result.Decode(serialization.NewReader(data, limits), &result) == nil {
			require.True(t, result.Valid())
		}
	})
}
