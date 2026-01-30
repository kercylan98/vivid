package chain

import "context"

type ChainsVoid struct {
	chains []any
}

func NewVoid() *ChainsVoid {
	return &ChainsVoid{
		chains: make([]any, 0),
	}
}

func (c *ChainsVoid) Append(chain ChainVoid) *ChainsVoid {
	c.chains = append(c.chains, chain)
	return c
}

func (c *ChainsVoid) AppendContext(chain ContextChainVoid) *ChainsVoid {
	c.chains = append(c.chains, chain)
	return c
}

func (c *ChainsVoid) Run() {
	for _, chain := range c.chains {
		switch chain := chain.(type) {
		case ChainVoid:
			chain.Run()
		case ContextChainVoid:
			chain.Run(context.Background())
		}
	}
}

type ChainVoid interface {
	Run()
}

type ChainVoidFN func()

func (fn ChainVoidFN) Run() {
	fn()
}

type ContextChainVoid interface {
	Run(ctx context.Context)
}

type ContextChainVoidFN func(ctx context.Context)

func (fn ContextChainVoidFN) Run(ctx context.Context) {
	fn(ctx)
}
