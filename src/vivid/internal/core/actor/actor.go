package actor

// Actor 是由用户定义的 Actor 行为接口，他将对 Actor 所收到的消息进行处理
type Actor interface {
	// OnReceive 当 Actor 接收到消息时，将调用此方法
	OnReceive(ctx Context)
}
