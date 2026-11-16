package remoting

import "github.com/kercylan98/vivid"

const defaultEndpointMaxPendingEnvelops = 1024

type endpointBufferPolicy struct {
	maxPendingEnvelops int
}

func newEndpointBufferPolicy(opts *vivid.ActorSystemRemotingOptions) endpointBufferPolicy {
	policy := endpointBufferPolicy{
		maxPendingEnvelops: defaultEndpointMaxPendingEnvelops,
	}
	if opts == nil {
		return policy
	}
	if opts.MaxPendingEnvelops > 0 {
		policy.maxPendingEnvelops = opts.MaxPendingEnvelops
	}
	return policy
}

type endpointOutboundBuffer struct {
	limit int
	items []vivid.Envelop
}

func newEndpointOutboundBuffer(policy endpointBufferPolicy) *endpointOutboundBuffer {
	return &endpointOutboundBuffer{
		limit: policy.maxPendingEnvelops,
	}
}

func (b *endpointOutboundBuffer) Push(envelop vivid.Envelop) error {
	if envelop == nil {
		return vivid.ErrorIllegalArgument.WithMessage("endpoint outbound envelop is nil")
	}
	if b.limit > 0 && len(b.items) >= b.limit {
		return vivid.ErrorRemotingPendingEnvelopsFull
	}
	b.items = append(b.items, envelop)
	return nil
}

func (b *endpointOutboundBuffer) Peek() vivid.Envelop {
	if len(b.items) == 0 {
		return nil
	}
	return b.items[0]
}

func (b *endpointOutboundBuffer) Pop() vivid.Envelop {
	if len(b.items) == 0 {
		return nil
	}
	item := b.items[0]
	b.items[0] = nil
	b.items = b.items[1:]
	if len(b.items) == 0 {
		b.items = nil
	}
	return item
}

func (b *endpointOutboundBuffer) Len() int {
	return len(b.items)
}

func (b *endpointOutboundBuffer) Empty() bool {
	return len(b.items) == 0
}

func (b *endpointOutboundBuffer) FailAll(handler NetworkEnvelopHandler) {
	items := b.items
	b.items = nil
	for _, envelop := range items {
		failOutboundEnvelop(handler, envelop)
	}
}
