package vivid

import "time"

type AccidentRecord interface {
	accidentRecordInternal

	// GetPrimeCulprit 获取事故元凶，即导致事故的消息发送人
	GetPrimeCulprit() ActorRef

	// GetVictim 获取事故受害者，即接收到导致事故的消息的 Actor
	GetVictim() ActorRef

	// GetMessage 获取导致事故的消息
	GetMessage() Message

	// GetReason 获取事故原因
	GetReason() Message

	// GetStack 获取事件堆栈
	GetStack() []byte

	// Kill 立即停止目标 Actor 继续运行
	Kill(ref ActorRef, reason string)

	// PoisonKill 在目标 Actor 处理完剩余消息后停止其运行
	PoisonKill(ref ActorRef, reason string)

	// Resume 忽略本条消息并恢复事故受害者的运行
	Resume()

	// Restart 重启目标 Actor
	Restart(ref ActorRef, gracefully bool, reason string)

	// Escalate 将事故升级至上级 Actor 处理
	Escalate()

	// GetRestartCount 获取重启次数
	GetRestartCount() int
}

type accidentRecordInternal interface {
	setResponsiblePerson(ctx ActorContext)

	recordRestartFailed(primeCulprit, victim ActorRef, message, reason Message, stack []byte)
}

func newAccidentRecord(mailbox Mailbox, primeCulprit, victim ActorRef, message, reason Message, stack []byte) AccidentRecord {
	return &accidentRecord{
		Mailbox:      mailbox,
		PrimeCulprit: primeCulprit,
		Victim:       victim,
		Message:      message,
		Reason:       reason,
		Stack:        stack,
	}
}

// AccidentRecord 事故记录
type accidentRecord struct {
	Mailbox           Mailbox      // 事故受害者的邮箱
	responsiblePerson ActorContext // 当前责任人上下文
	PrimeCulprit      ActorRef     // 事故元凶
	Victim            ActorRef     // 事故受害者
	Message           Message      // 造成事故发生的消息
	Reason            Message      // 事故原因
	Stack             []byte       // 事件堆栈
	Escalated         bool         // 是否已经升级
	RestartTimes      []time.Time  // 重启时间
}

func (record *accidentRecord) GetPrimeCulprit() ActorRef {
	return record.PrimeCulprit
}

func (record *accidentRecord) GetVictim() ActorRef {
	return record.Victim
}

func (record *accidentRecord) GetMessage() Message {
	return record.Message
}

func (record *accidentRecord) GetReason() Message {
	return record.Reason
}

func (record *accidentRecord) GetStack() []byte {
	return record.Stack
}

func (record *accidentRecord) Kill(ref ActorRef, reason string) {
	record.responsiblePerson.Kill(ref, reason)
}

func (record *accidentRecord) PoisonKill(ref ActorRef, reason string) {
	record.Mailbox.Resume()
	record.responsiblePerson.PoisonKill(ref, reason)
}

func (record *accidentRecord) Resume() {
	record.Mailbox.Resume()
}

func (record *accidentRecord) Restart(ref ActorRef, gracefully bool, reason string) {
	record.Mailbox.Resume()
	record.responsiblePerson.Restart(ref, gracefully, reason)
}

func (record *accidentRecord) Escalate() {
	if record.Escalated {
		return
	}
	record.Escalated = true
	record.responsiblePerson.tell(record.responsiblePerson.Parent(), record, SystemMessage)
}

func (record *accidentRecord) setResponsiblePerson(ctx ActorContext) {
	record.responsiblePerson = ctx
	record.Escalated = false
}

func (record *accidentRecord) GetRestartCount() int {
	return len(record.RestartTimes)
}

func (record *accidentRecord) recordRestartFailed(primeCulprit, victim ActorRef, message, reason Message, stack []byte) {
	record.RestartTimes = append(record.RestartTimes, time.Now())
	record.PrimeCulprit = primeCulprit
	record.Victim = victim
	record.Message = message
	record.Reason = reason
	record.Stack = stack
}
