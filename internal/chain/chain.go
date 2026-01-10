package chain

import "context"

type Chains struct {
	chains []any
}

type ChainsOptions struct {
	Context context.Context
}

type ChainsOption func(options *ChainsOptions)

func WithOptions(options ChainsOptions) ChainsOption {
	return func(opts *ChainsOptions) {
		*opts = options
	}
}

func WithContext(context context.Context) ChainsOption {
	return func(opts *ChainsOptions) {
		opts.Context = context
	}
}

func New(options ...ChainsOption) *Chains {
	opts := ChainsOptions{
		Context: context.Background(),
	}
	for _, option := range options {
		option(&opts)
	}
	return &Chains{
		chains: make([]any, 0),
	}
}

func (c *Chains) Append(chain Chain) *Chains {
	c.chains = append(c.chains, chain)
	return c
}

func (c *Chains) AppendContext(chain ContextChain) *Chains {
	c.chains = append(c.chains, chain)
	return c
}

func (c *Chains) Run() error {
	for _, chain := range c.chains {
		switch chain := chain.(type) {
		case Chain:
			if err := chain.Run(); err != nil {
				return err
			}
		case ContextChain:
			if err := chain.Run(context.Background()); err != nil {
				return err
			}
		}
	}
	return nil
}

type Chain interface {
	Run() error
}

type ChainFN func() error

func (fn ChainFN) Run() error {
	return fn()
}

type ContextChain interface {
	Run(ctx context.Context) error
}

type ContextChainFN func(ctx context.Context) error

func (fn ContextChainFN) Run(ctx context.Context) error {
	return fn(ctx)
}
