package vivid

// Mailbox 是消息邮箱的接口，定义了消息的入队操作。
// 每个 Mailbox 实现都应当保证 Enqueue 的并发和顺序性由具体实现决定，通常用于 actor 框架中接收和调度消息。
// Mailbox 的实现需要确保线程安全，能够正确处理高并发情况下的消息投递和消费。
type Mailbox interface {
	// Enqueue 用于将消息信封入队。
	// 参数 envelop 为待入队的消息信封，该信封一般实现了 Envelop 接口。
	Enqueue(envelop Envelop)
}

// Envelop 定义了消息传递过程中的信封接口。
// 一个 Envelop 包含了消息的发送者、被代理者（用于 Future 或特殊传递场景）、消息实体，以及是否为系统消息。
// 该接口允许 actor 系统在传递消息时区分系统消息和普通消息，便于实现优先级调度等机制。
type Envelop interface {
	// System 返回该消息是否为系统消息。
	System() bool
	// Agent 返回该消息关联的代理 ActorRef，通常在 ask/future 等场景下使用。
	Agent() ActorRef
	// Sender 返回消息的发送者 ActorRef。
	Sender() ActorRef
	// Message 返回消息体。
	Message() Message
	// Receiver 接收人
	Receiver() ActorRef
}

// EnvelopHandler 定义了处理消息信封的接口。
// EnvelopHandler 负责将接收到的消息信封进行业务逻辑处理，通常由 Mailbox 持有并调用。
type EnvelopHandler interface {
	// HandleEnvelop 用于处理传入的消息信封。
	// 参数 envelop 是实现了 Envelop 接口的消息信封。
	HandleEnvelop(envelop Envelop)
}
