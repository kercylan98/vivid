package vivid

// Future[T] 为 Actor 模式下异步请求-响应的结果占位对象（泛型）。
// 用于异步消息通信（如 Ask）场景，支持并发安全、等待应答、超时控制与消息管道等能力。
// T 为业务自定义的期望响应消息类型，提高类型安全与易用性。
type Future[T any] interface {
	// Close 主动关闭 Future，标记当前 Future 已完成或异常，不再接受后续结果。
	// 参数:
	//   - err: 关闭原因或异常信息，若为 nil 表示正常关闭。
	// 常见用于取消请求、回收资源或异常终止。关闭后 Wait/Result 等会返回该错误。
	Close(err error)

	// Result 会阻塞等待 Future 完成（应答消息返回或发生超时/异常），并返回消息结果与错误信息。
	// 返回值:
	//   - T: 业务约定的数据类型（泛型），如超时或异常时返回 T 的零值
	//   - error: 可能的异常（如超时、类型不符、手动关闭等）
	// 若 Future 已被关闭，直接返回最终错误。否则阻塞等待直至结果到达。
	Result() (T, error)

	// Wait 阻塞等待 Future 结束（无论成功/失败），仅返回异常信息，不关心实际结果。
	// 常用于只需同步化确认完成状态、不关心消息内容的场景。
	// 返回值:
	//   - error: Future 结束对应的异常信息（正常结束则为 nil）
	Wait() error

	// PipeTo 在 Future 得到结果时，将结果以 *PipeResult（Message + Error）形式转发给指定的 ActorRef，可跨网络序列化。
	// 参数:
	//   - forwarders: 结果需要进一步转发给的其他 ActorRef 列表
	PipeTo(forwarders ActorRefs) error
}
