package vivid

import (
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
	"time"
)

var _ AccidentSnapshot = (*accidentSnapshot)(nil)

type Supervisor interface {
	Decision(snapshot AccidentSnapshot)
}

type SupervisorFN func(snapshot AccidentSnapshot)

func (f SupervisorFN) Decision(snapshot AccidentSnapshot) {
	f(snapshot)
}

type AccidentSnapshot interface {
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
	Kill(ref ActorRef, reason ...string)

	// PoisonKill 在目标 Actor 处理完剩余消息后停止其运行
	PoisonKill(ref ActorRef, reason ...string)

	// Resume 忽略本条消息并恢复事故受害者的运行
	Resume()

	// Restart 重启目标 Actor，并在重启后继续处理剩余消息
	Restart(ref ActorRef, reason ...string)

	// ExponentialBackoffRestart 退避指数重启
	ExponentialBackoffRestart(ref ActorRef, restartCount int, baseDelay, maxDelay time.Duration, multiplier, randomization float64, reason ...string)

	// Escalate 将事故升级至上级 Actor 处理
	Escalate()
}

func newAccidentSnapshot(snapshot actor.Snapshot) AccidentSnapshot {
	return &accidentSnapshot{snapshot: snapshot}
}

type accidentSnapshot struct {
	snapshot actor.Snapshot
}

func (a *accidentSnapshot) ActorContext() ActorContext {
	return newActorContext(a.snapshot.ActorContext())
}

func (a *accidentSnapshot) GetPrimeCulprit() ActorRef {
	return a.snapshot.GetPrimeCulprit()
}

func (a *accidentSnapshot) GetVictim() ActorRef {
	return a.snapshot.GetVictim()
}

func (a *accidentSnapshot) GetMessage() Message {
	return a.snapshot.GetMessage()
}

func (a *accidentSnapshot) GetReason() Message {
	return a.snapshot.GetReason()
}

func (a *accidentSnapshot) GetStack() []byte {
	return a.snapshot.GetStack()
}

func (a *accidentSnapshot) GetRestartCount() int {
	return a.snapshot.GetRestartCount()
}

func (a *accidentSnapshot) Kill(ref ActorRef, reason ...string) {
	a.snapshot.Kill(ref.(actor.Ref), reason...)
}

func (a *accidentSnapshot) PoisonKill(ref ActorRef, reason ...string) {
	a.snapshot.PoisonKill(ref.(actor.Ref), reason...)
}

func (a *accidentSnapshot) Resume() {
	a.snapshot.Resume()
}

func (a *accidentSnapshot) Restart(ref ActorRef, reason ...string) {
	a.snapshot.Restart(ref.(actor.Ref), reason...)
}

func (a *accidentSnapshot) ExponentialBackoffRestart(ref ActorRef, restartCount int, baseDelay, maxDelay time.Duration, multiplier, randomization float64, reason ...string) {
	a.snapshot.ExponentialBackoffRestart(ref.(actor.Ref), restartCount, baseDelay, maxDelay, multiplier, randomization, reason...)
}

func (a *accidentSnapshot) Escalate() {
	a.snapshot.Escalate()
}
