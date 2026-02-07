package chain_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kercylan98/vivid/internal/chain"
	"github.com/stretchr/testify/assert"
)

func TestWithOptions(t *testing.T) {
	t.Run("sets options", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "key", "value")
		opts := chain.ChainsOptions{Context: ctx}
		opt := chain.WithOptions(opts)
		assert.NotNil(t, opt)
		var result chain.ChainsOptions
		opt(&result)
		assert.Equal(t, ctx, result.Context)
	})
}

func TestWithContext(t *testing.T) {
	t.Run("sets context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "key", "value")
		opt := chain.WithContext(ctx)
		assert.NotNil(t, opt)
		var opts chain.ChainsOptions
		opt(&opts)
		assert.Equal(t, ctx, opts.Context)
	})
}

func TestNew(t *testing.T) {
	t.Run("default options", func(t *testing.T) {
		c := chain.New()
		assert.NotNil(t, c)
	})

	t.Run("with options", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "key", "value")
		c := chain.New(chain.WithOptions(chain.ChainsOptions{Context: ctx}))
		assert.NotNil(t, c)
	})

	t.Run("with context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "key", "value")
		c := chain.New(chain.WithContext(ctx))
		assert.NotNil(t, c)
	})
}

func TestChains_Append(t *testing.T) {
	t.Run("appends chain and returns self", func(t *testing.T) {
		c := chain.New()
		called := false
		fn := chain.ChainFN(func() error {
			called = true
			return nil
		})
		result := c.Append(fn)
		assert.Same(t, c, result)
		assert.NoError(t, c.Run())
		assert.True(t, called)
	})
}

func TestChains_AppendContext(t *testing.T) {
	t.Run("appends context chain and returns self", func(t *testing.T) {
		c := chain.New()
		called := false
		fn := chain.ContextChainFN(func(ctx context.Context) error {
			called = true
			assert.NotNil(t, ctx)
			return nil
		})
		result := c.AppendContext(fn)
		assert.Same(t, c, result)
		assert.NoError(t, c.Run())
		assert.True(t, called)
	})
}

func TestChains_Run(t *testing.T) {
	t.Run("empty chains returns nil", func(t *testing.T) {
		c := chain.New()
		assert.NoError(t, c.Run())
	})

	t.Run("chain returns error", func(t *testing.T) {
		errExpected := errors.New("chain error")
		c := chain.New()
		c.Append(chain.ChainFN(func() error { return errExpected }))
		err := c.Run()
		assert.ErrorIs(t, err, errExpected)
	})

	t.Run("context chain returns error", func(t *testing.T) {
		errExpected := errors.New("context chain error")
		c := chain.New()
		c.AppendContext(chain.ContextChainFN(func(ctx context.Context) error { return errExpected }))
		err := c.Run()
		assert.ErrorIs(t, err, errExpected)
	})

	t.Run("mixed chains run in order", func(t *testing.T) {
		var order []string
		c := chain.New()
		c.Append(chain.ChainFN(func() error {
			order = append(order, "chain1")
			return nil
		}))
		c.AppendContext(chain.ContextChainFN(func(ctx context.Context) error {
			order = append(order, "context1")
			return nil
		}))
		c.Append(chain.ChainFN(func() error {
			order = append(order, "chain2")
			return nil
		}))
		assert.NoError(t, c.Run())
		assert.Equal(t, []string{"chain1", "context1", "chain2"}, order)
	})

	t.Run("stops on first chain error", func(t *testing.T) {
		errExpected := errors.New("first error")
		called := false
		c := chain.New()
		c.Append(chain.ChainFN(func() error { return errExpected }))
		c.Append(chain.ChainFN(func() error {
			called = true
			return nil
		}))
		err := c.Run()
		assert.ErrorIs(t, err, errExpected)
		assert.False(t, called)
	})
}

func TestChainFN_Run(t *testing.T) {
	t.Run("executes function", func(t *testing.T) {
		called := false
		fn := chain.ChainFN(func() error {
			called = true
			return nil
		})
		assert.NoError(t, fn.Run())
		assert.True(t, called)
	})

	t.Run("returns error", func(t *testing.T) {
		errExpected := errors.New("fn error")
		fn := chain.ChainFN(func() error { return errExpected })
		assert.ErrorIs(t, fn.Run(), errExpected)
	})
}

func TestContextChainFN_Run(t *testing.T) {
	t.Run("executes with context", func(t *testing.T) {
		called := false
		ctx := context.WithValue(context.Background(), "k", "v")
		fn := chain.ContextChainFN(func(c context.Context) error {
			called = true
			assert.Equal(t, "v", c.Value("k"))
			return nil
		})
		assert.NoError(t, fn.Run(ctx))
		assert.True(t, called)
	})

	t.Run("returns error", func(t *testing.T) {
		errExpected := errors.New("context fn error")
		fn := chain.ContextChainFN(func(context.Context) error { return errExpected })
		assert.ErrorIs(t, fn.Run(context.Background()), errExpected)
	})
}
