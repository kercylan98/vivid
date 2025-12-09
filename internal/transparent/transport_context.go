package transparent

import "github.com/kercylan98/vivid"

// TransportContext 是透明传输的上下文接口，通过 TransportContext 可以实现透明传输的消息处理。
type TransportContext interface {
	DeliverEnvelop(envelop vivid.Envelop)
}
