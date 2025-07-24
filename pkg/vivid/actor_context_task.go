package vivid

type ActorContextTask interface {
	Receive(context TaskContext)
}

type ActorContextTaskFN func(context TaskContext)

func (fn ActorContextTaskFN) Receive(context TaskContext) {
	fn(context)
}

func newTaskContext(context *actorContext, task ActorContextTask) *taskContext {
	return &taskContext{
		actorContext: context,
		task:         task,
		sender:       context.Sender(),
		message:      context.Message(),
	}
}

type TaskContext interface {
	ActorContext

	// Sender 该函数为 ActorContext.Sender 的别名，它使用的上下文是当前任务所关联时刻的原始上下文。
	Sender() ActorRef

	// Message 该函数为 ActorContext.Message 的别名，它使用的上下文是当前任务所关联时刻的原始上下文。
	Message() Message

	// Reply 该函数为 ActorContext.Reply 的别名，它使用的上下文是当前任务所关联时刻的原始上下文。
	Reply(message Message)
}

type taskContext struct {
	*actorContext
	task    ActorContextTask // 当前任务
	sender  ActorRef         // 创建任务时刻的原始发送人
	message Message          // 创建任务时刻的原始消息
}

func (c *taskContext) Sender() ActorRef {
	return c.sender
}

func (c *taskContext) Message() Message {
	return c.message
}

func (c *taskContext) Reply(message Message) {
	c.Tell(c.sender, message)
}

func (c *taskContext) handle() {
	c.withFatalRecover(func() {
		c.task.Receive(c)
	})
}
