package sugar_test

import (
	"testing"

	"github.com/kercylan98/vivid/internal/sugar"
	"github.com/stretchr/testify/assert"
)

func TestFirstOrDefault(t *testing.T) {
	t.Run("case1", func(t *testing.T) {
		assert.Equal(t, 1, sugar.FirstOrDefault([]int{1}, 0))
	})

	t.Run("case2", func(t *testing.T) {
		assert.Equal(t, 1, sugar.FirstOrDefault([]int{1, 2, 3}, 0))
	})

	t.Run("case3", func(t *testing.T) {
		assert.Equal(t, 5, sugar.FirstOrDefault([]int{}, 5))
	})

	t.Run("case4", func(t *testing.T) {
		assert.Equal(t, 5, sugar.FirstOrDefault(nil, 5))
	})
}

func TestMax(t *testing.T) {
	t.Run("case1", func(t *testing.T) {
		assert.Equal(t, sugar.Max(1, 2), 2)
	})

	t.Run("case2", func(t *testing.T) {
		assert.Equal(t, sugar.Max(1, 1), 1)
	})
}

func TestMin(t *testing.T) {
	t.Run("case1", func(t *testing.T) {
		assert.Equal(t, sugar.Min(1, 2), 1)
	})

	t.Run("case2", func(t *testing.T) {
		assert.Equal(t, sugar.Min(1, 1), 1)
	})
}
