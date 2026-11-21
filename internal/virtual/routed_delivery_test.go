package virtual_test

import (
	"testing"

	"github.com/kercylan98/vivid/internal/serialization"
	"github.com/kercylan98/vivid/internal/virtual"
	"github.com/stretchr/testify/require"
)

func routedCodec(t testing.TB) (*serialization.VividCodec, *virtual.RoutedDeliveryCodec) {
	t.Helper()
	c := serialization.NewVividCodec(nil)
	route := virtual.NewRoutedDeliveryCodec(c)
	require.NoError(t, c.RegisterMessageWithEncoderAndDecoder("virtual", "route", new(virtual.RoutedDelivery), route, route))
	require.NoError(t, c.RegisterMessage("virtual", "identity", new(virtual.Identity)))
	require.NoError(t, c.RegisterMessage("virtual", "result", new(virtual.DeliveryResult)))
	return c, route
}

func routedMessage() *virtual.RoutedDelivery {
	return &virtual.RoutedDelivery{Group: [32]byte{1}, Owner: [16]byte{2}, Generation: 3, Kind: "worker", Name: "one", Sender: "127.0.0.1:9000/sender", Message: virtual.NewIdentity("payload", "独立内容")}
}

func TestRoutedDelivery_CodecAndTruncation(t *testing.T) {
	c, route := routedCodec(t)
	m := routedMessage()
	w := serialization.NewWriter()
	require.NoError(t, c.Encode(w, m))
	data := w.Bytes()
	value, err := c.Decode(serialization.NewReader(data))
	require.NoError(t, err)
	decoded := value.(*virtual.RoutedDelivery)
	require.Equal(t, m, decoded)
	require.NotSame(t, m.Message, decoded.Message)
	for end := range len(data) {
		_, err = c.Decode(serialization.NewReader(data[:end]))
		require.Error(t, err, "截断位置 %d", end)
	}
	w = serialization.NewWriter()
	require.NoError(t, route.Encode(w, m))
	for end := range len(w.Bytes()) {
		old := *m
		require.Error(t, route.Decode(serialization.NewReader(w.Bytes()[:end]), &old))
		require.Equal(t, *m, old, "失败不替换原目标")
	}
	result := &virtual.DeliveryResult{Group: m.Group, Owner: m.Owner, Generation: m.Generation, Kind: m.Kind, Name: m.Name, Status: virtual.DeliveryAccepted}
	w = serialization.NewWriter()
	require.NoError(t, result.Encode(w, result))
	var output virtual.DeliveryResult
	require.NoError(t, output.Decode(serialization.NewReader(w.Bytes()), &output))
	require.Equal(t, *result, output)
	for end := range len(w.Bytes()) {
		old := *result
		require.Error(t, old.Decode(serialization.NewReader(w.Bytes()[:end]), &old))
		require.Equal(t, *result, old)
	}
	data = append([]byte(nil), w.Bytes()...)
	data[0] = 2
	require.ErrorIs(t, output.Decode(serialization.NewReader(data), &output), virtual.ErrInvalidRoutedDelivery)
	data[0], data[len(data)-1] = 1, 255
	require.ErrorIs(t, output.Decode(serialization.NewReader(data), &output), virtual.ErrInvalidRoutedDelivery)
}

func TestRoutedDelivery_ValidationAndSharedBudget(t *testing.T) {
	c, route := routedCodec(t)
	for _, change := range []func(*virtual.RoutedDelivery){
		func(m *virtual.RoutedDelivery) { m.Group = [32]byte{} },
		func(m *virtual.RoutedDelivery) { m.Owner = [16]byte{} },
		func(m *virtual.RoutedDelivery) { m.Generation = 0 },
		func(m *virtual.RoutedDelivery) { m.Kind = "\xff" },
		func(m *virtual.RoutedDelivery) { m.Sender = "" },
		func(m *virtual.RoutedDelivery) { m.Message = (*virtual.Identity)(nil) },
		func(m *virtual.RoutedDelivery) { m.Message = routedMessage() },
	} {
		m := routedMessage()
		change(m)
		require.False(t, m.Valid())
		require.Error(t, c.Encode(serialization.NewWriter(), m))
	}
	m := routedMessage()
	w := serialization.NewWriter()
	require.NoError(t, c.Encode(w, m))
	_, err := c.Decode(serialization.NewReader(w.Bytes(), serialization.DecodeLimits{MaxDepth: 1}))
	require.Error(t, err, "内层消息沿用外层深度预算")
	w = serialization.NewWriter()
	require.NoError(t, route.Encode(w, m))
	data := append(append([]byte(nil), w.Bytes()...), w.Bytes()...)
	// 只允许一份 header 和 payload 的字符串申请，第二份不能重置累计额度。
	reader := serialization.NewReader(data, serialization.DecodeLimits{MaxAllocationBytes: 128})
	var decoded virtual.RoutedDelivery
	require.NoError(t, route.Decode(reader, &decoded))
	require.Error(t, route.Decode(reader, &decoded))
	// 非法 header 在调用业务解码器前拒绝。
	bad := serialization.NewWriter().Write(uint8(1), [32]byte{}, m.Owner, m.Generation, m.Kind, m.Name, m.Sender)
	require.ErrorIs(t, route.Decode(serialization.NewReader(bad.Bytes()), &decoded), virtual.ErrInvalidRoutedDelivery)
	reader = serialization.NewReader(data, serialization.DecodeLimits{MaxElements: 95})
	require.NoError(t, route.Decode(reader, &decoded))
	require.ErrorIs(t, route.Decode(reader, &decoded), serialization.ErrDecodeLimit)
	// 手工构造线上嵌套包装，不能依赖正常编码入口的拒绝检查。
	nested := serialization.NewWriter().Write(uint8(1), m.Group, m.Owner, m.Generation, m.Kind, m.Name, m.Sender)
	require.NoError(t, c.Encode(nested, m))
	require.ErrorIs(t, route.Decode(serialization.NewReader(nested.Bytes()), &decoded), virtual.ErrInvalidRoutedDelivery)
}

func FuzzRoutedDeliveryDecode(f *testing.F) {
	c, _ := routedCodec(f)
	w := serialization.NewWriter()
	require.NoError(f, c.Encode(w, routedMessage()))
	f.Add(w.Bytes())
	f.Add([]byte{1})
	f.Fuzz(func(t *testing.T, data []byte) {
		decoded, err := c.Decode(serialization.NewReader(data, serialization.DecodeLimits{MaxAllocationBytes: 64 << 10, MaxDepth: 16}))
		if err == nil {
			switch m := decoded.(type) {
			case *virtual.RoutedDelivery:
				require.True(t, m.Valid())
			}
		}
	})
}
