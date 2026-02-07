package chain_test

import (
	"context"
	"testing"

	"github.com/kercylan98/vivid/internal/chain"
	"github.com/stretchr/testify/assert"
)

func TestNewVoid(t *testing.T) {
	t.Run("returns new ChainsVoid", func(t *testing.T) {
		c := chain.NewVoid()
		assert.NotNil(t, c)
	})
}

func TestChainsVoid_Append(t *testing.T) {
	t.Run("appends chain and returns self", func(t *testing.T) {
		c := chain.NewVoid()
		called := false
		fn := chain.VoidFN(func() {
			called = true
		})
		result := c.Append(fn)
		assert.Same(t, c, result)
		c.Run()
		assert.True(t, called)
	})
}

func TestChainsVoid_AppendContext(t *testing.T) {
	t.Run("appends context chain and returns self", func(t *testing.T) {
		c := chain.NewVoid()
		called := false
		fn := chain.ContextChainVoidFN(func(ctx context.Context) {
			called = true
			assert.NotNil(t, ctx)
		})
		result := c.AppendContext(fn)
		assert.Same(t, c, result)
		c.Run()
		assert.True(t, called)
	})
}

func TestChainsVoid_Run(t *testing.T) {
	t.Run("empty chains completes", func(t *testing.T) {
		c := chain.NewVoid()
		assert.NotPanics(t, c.Run)
	})

	t.Run("mixed chains run in order", func(t *testing.T) {
		var order []string
		c := chain.NewVoid()
		c.Append(chain.VoidFN(func() {
			order = append(order, "void1")
		}))
		c.AppendContext(chain.ContextChainVoidFN(func(ctx context.Context) {
			order = append(order, "context1")
		}))
		c.Append(chain.VoidFN(func() {
			order = append(order, "void2")
		}))
		c.Run()
		assert.Equal(t, []string{"void1", "context1", "void2"}, order)
	})
}

func TestChainVoidFN_Run(t *testing.T) {
	t.Run("executes function", func(t *testing.T) {
		called := false
		fn := chain.VoidFN(func() {
			called = true
		})
		fn.Run()
		assert.True(t, called)
	})
}

func TestContextChainVoidFN_Run(t *testing.T) {
	t.Run("executes with context", func(t *testing.T) {
		called := false
		ctx := context.WithValue(context.Background(), "k", "v")
		fn := chain.ContextChainVoidFN(func(c context.Context) {
			called = true
			assert.Equal(t, "v", c.Value("k"))
		})
		fn.Run(ctx)
		assert.True(t, called)
	})
}
