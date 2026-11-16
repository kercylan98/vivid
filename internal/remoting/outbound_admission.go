package remoting

import (
	"sync"
	"sync/atomic"

	"github.com/kercylan98/vivid"
)

// OutboundAdmission 在消息进入远程邮箱前保留名额，覆盖排队、重连与写入的整个过程。
// 同一系统的所有远程邮箱共享该实例，回调在锁外执行。
type OutboundAdmission struct {
	mu         sync.Mutex
	pending    map[*outboundEnvelop]struct{}
	addresses  map[string]int
	perAddress int
	total      int
	stopped    bool
	handler    FailedEnvelopHandler
	onRejected func(vivid.Envelop, error)
}

func NewOutboundAdmission(options vivid.ActorSystemRemotingOptions, handler FailedEnvelopHandler, onRejected func(vivid.Envelop, error)) *OutboundAdmission {
	defaults := vivid.NewActorSystemRemotingOptions()
	if options.MaxPendingEnvelops <= 0 {
		options.MaxPendingEnvelops = defaults.MaxPendingEnvelops
	}
	if options.MaxTotalPendingEnvelops <= 0 {
		options.MaxTotalPendingEnvelops = defaults.MaxTotalPendingEnvelops
	}
	return &OutboundAdmission{
		pending:    make(map[*outboundEnvelop]struct{}),
		addresses:  make(map[string]int),
		perAddress: options.MaxPendingEnvelops,
		total:      options.MaxTotalPendingEnvelops,
		handler:    handler,
		onRejected: onRejected,
	}
}

// outboundEnvelop 只用于本地传输调度，线上编码仍使用原始信封字段。
// 已完成的信封不复用，迟到结果不能释放其他消息的名额。
type outboundEnvelop struct {
	vivid.Envelop
	admission *OutboundAdmission
	address   string // 本次准入占用的地址，仅用于计数与释放，不作为位置缓存
	done      atomic.Bool
}

func (a *OutboundAdmission) reserve(envelop vivid.Envelop) (*outboundEnvelop, error) {
	address := ensureEnvelopReceiverAddress(envelop)
	if address == "" {
		return nil, vivid.ErrorIllegalArgument.WithMessage("remote receiver address is empty")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.stopped {
		return nil, vivid.ErrorRemotingStopped
	}
	if a.addresses[address] >= a.perAddress {
		return nil, vivid.ErrorRemotingPendingEnvelopsFull
	}
	if len(a.pending) >= a.total {
		return nil, vivid.ErrorRemotingTotalPendingEnvelopsFull
	}
	pending := &outboundEnvelop{Envelop: envelop, admission: a, address: address}
	a.pending[pending] = struct{}{}
	a.addresses[address]++
	return pending, nil
}

func (a *OutboundAdmission) reject(envelop vivid.Envelop, err error) {
	if a.onRejected != nil {
		a.onRejected(envelop, err)
	}
	if a.handler != nil {
		a.handler.HandleFailedRemotingEnvelop(envelop)
	}
}

// Stop 处理尚未到达端点的信封；后续端点清理和迟到结果不会重复失败回调。
func (a *OutboundAdmission) Stop() {
	a.mu.Lock()
	a.stopped = true
	pending := make([]*outboundEnvelop, 0, len(a.pending))
	for envelop := range a.pending {
		pending = append(pending, envelop)
	}
	a.mu.Unlock()
	for _, envelop := range pending {
		failOutboundEnvelop(a.handler, envelop)
	}
}

func completeOutboundEnvelop(envelop vivid.Envelop) (vivid.Envelop, bool) {
	pending, ok := envelop.(*outboundEnvelop)
	if !ok {
		return envelop, true
	}
	a := pending.admission
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, exists := a.pending[pending]; !exists {
		return pending.Envelop, false
	}
	delete(a.pending, pending)
	pending.done.Store(true)
	a.addresses[pending.address]--
	if a.addresses[pending.address] == 0 {
		delete(a.addresses, pending.address)
	}
	return pending.Envelop, true
}

func outboundEnvelopActive(envelop vivid.Envelop) bool {
	pending, ok := envelop.(*outboundEnvelop)
	return !ok || !pending.done.Load()
}

func failOutboundEnvelop(handler FailedEnvelopHandler, envelop vivid.Envelop) {
	if original, completed := completeOutboundEnvelop(envelop); completed && handler != nil {
		handler.HandleFailedRemotingEnvelop(original)
	}
}

// FailOutboundMessage 在内部 Actor 已终止而消息尚未处理时归还名额，并保留原始死信。
func FailOutboundMessage(message vivid.Message) bool {
	pending, ok := message.(*outboundEnvelop)
	if !ok {
		return false
	}
	failOutboundEnvelop(pending.admission.handler, pending)
	return true
}
