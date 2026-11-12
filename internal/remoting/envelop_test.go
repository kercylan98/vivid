package remoting

import (
	"testing"

	"github.com/kercylan98/vivid/internal/serialization"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type budgetExternalCodec struct{}

func (budgetExternalCodec) Encode(w *serialization.Writer, message any) error {
	w.WriteString(message.(string))
	return w.Err()
}

func (budgetExternalCodec) Decode(r *serialization.Reader) (any, error) {
	var message string
	err := r.ReadString(&message)
	return message, err
}

func TestDecodeEnvelop_SharedBudget(t *testing.T) {
	// 地址和消息体各占两个字节，额度不足时不能在进入 Codec 后重置。
	w := serialization.NewWriter().Write(false, "aa", "bb", "", "cc")
	codec := serialization.NewVividCodec(budgetExternalCodec{}, serialization.DecodeLimits{MaxAllocationBytes: 5})
	_, sender, receiver, _, err := decodeEnvelop(codec, w.Bytes())
	require.ErrorIs(t, err, serialization.ErrDecodeLimit)
	assert.Equal(t, "aa", sender)
	assert.Equal(t, "bb", receiver)
	codec = serialization.NewVividCodec(budgetExternalCodec{}, serialization.DecodeLimits{MaxAllocationBytes: 6})
	for i := 0; i < 2; i++ {
		_, _, _, message, err := decodeEnvelop(codec, w.Bytes())
		require.NoError(t, err)
		assert.Equal(t, "cc", message)
	}
	// 单独的 Reader 保留自身配置，Codec 不覆盖调用方的预算。
	r := serialization.NewReader(serialization.NewWriter().Write("", "cc").Bytes(), serialization.DecodeLimits{MaxAllocationBytes: 1})
	_, err = codec.Decode(r)
	require.ErrorIs(t, err, serialization.ErrDecodeLimit)
}
