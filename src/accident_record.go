package vivid

import (
	"fmt"
	"math"
	"math/rand/v2"
	"sync/atomic"
	"time"
)

type accidentFinished struct {
	AccidentRecord AccidentRecord
}

// AccidentRecord 事故记录信息，当事故发生时必须通过任一非 Get 方法来处理事故，当事故未被处理时将会自动进一步升级
type AccidentRecord interface {
	accidentRecordInternal

	// ActorContext 获取当前责任人上下文
	ActorContext() ActorContext

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

	// GetRestartCount 获取重启次数
	GetRestartCount() int

	// Kill 立即停止目标 Actor 继续运行
	Kill(ref ActorRef, reason string)

	// PoisonKill 在目标 Actor 处理完剩余消息后停止其运行
	PoisonKill(ref ActorRef, reason string)

	// Resume 忽略本条消息并恢复事故受害者的运行
	Resume()

	// Restart 重启目标 Actor，并在重启后继续处理剩余消息
	Restart(ref ActorRef, reason string)

	// ExponentialBackoffRestart 退避指数重启
	ExponentialBackoffRestart(ref ActorRef, reason string, restartCount int, baseDelay, maxDelay time.Duration, multiplier, randomization float64)

	// Escalate 将事故升级至上级 Actor 处理
	Escalate()
}

type accidentRecordInternal interface {
	setResponsiblePerson(ctx ActorContext)

	recordRestartFailed(primeCulprit, victim ActorRef, message, reason Message, stack []byte)

	isFinished() bool

	isDelayFinished() bool
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
	RestartTimes      []time.Time  // 重启时间
	Finished          atomic.Bool  // 是否已经处理完毕
	DelayFinished     bool         // 是否是延迟处理的（退避指数重启）
}

func (record *accidentRecord) ActorContext() ActorContext {
	return record.responsiblePerson
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
	if !record.Finished.CompareAndSwap(false, true) {
		return
	}
	record.responsiblePerson.Kill(ref, reason)
}

func (record *accidentRecord) PoisonKill(ref ActorRef, reason string) {
	if !record.Finished.CompareAndSwap(false, true) {
		return
	}
	record.Mailbox.Resume()
	record.responsiblePerson.PoisonKill(ref, reason)
}

func (record *accidentRecord) Resume() {
	if !record.Finished.CompareAndSwap(false, true) {
		return
	}
	record.Mailbox.Resume()
}

func (record *accidentRecord) Restart(ref ActorRef, reason string) {
	if !record.Finished.CompareAndSwap(false, true) {
		return
	}
	record.Mailbox.Resume()

	// 当意外发生后，Actor 的状态无法得到保证，需要在重启后继续处理剩余消息，所以不需要优雅重启
	record.responsiblePerson.Restart(ref, false, reason)
}

// ExponentialBackoffRestart 退避指数重启
func (record *accidentRecord) ExponentialBackoffRestart(ref ActorRef, reason string, restartCount int, baseDelay, maxDelay time.Duration, multiplier, randomization float64) {
	if !record.Finished.CompareAndSwap(false, true) {
		return
	}
	record.DelayFinished = true
	// 如果是重启，通过退避策略来控制重启次数，达到上限后停止。
	delay := float64(baseDelay) * math.Pow(multiplier, float64(record.GetRestartCount()))
	jitter := (rand.Float64() - 0.5) * randomization * float64(baseDelay)
	after := time.Duration(delay + jitter)
	if after > maxDelay {
		after = maxDelay
	}

	if count := record.GetRestartCount(); count >= restartCount {
		record.Finished.Store(false)
		record.Kill(ref, "supervisor: OnLaunch restart fail count limit")
		record.responsiblePerson.tell(record.responsiblePerson.Ref(), &accidentFinished{AccidentRecord: record}, SystemMessage)
	} else {
		// 使用当前责任人的定时器来执行重启操作
		key := fmt.Sprintf("supervisor:restart:%s:%d", record.GetVictim().GetPath(), time.Now().UnixMilli())
		record.ActorContext().accidentAfter(key, after, func(ctx ActorContext) {
			record.Finished.Store(false)
			record.Restart(ref, reason)
			record.responsiblePerson.tell(record.responsiblePerson.Ref(), &accidentFinished{AccidentRecord: record}, SystemMessage)
		})
	}
}

func (record *accidentRecord) Escalate() {
	if !record.Finished.CompareAndSwap(false, true) {
		return
	}
	record.responsiblePerson.tell(record.responsiblePerson.Parent(), record, SystemMessage)
}

func (record *accidentRecord) setResponsiblePerson(ctx ActorContext) {
	record.responsiblePerson = ctx
	record.Finished.Store(false)
	record.DelayFinished = false
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

func (record *accidentRecord) isFinished() bool {
	return record.Finished.Load()
}

func (record *accidentRecord) isDelayFinished() bool {
	return record.DelayFinished
}
