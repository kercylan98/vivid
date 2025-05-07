package actor

import (
	"github.com/kercylan98/vivid/src/vivid/internal/core"
	"time"
)

type AccidentSnapshotEnd struct {
	Snapshot AccidentSnapshot
}

// AccidentSnapshot 事故快照，当事故发生时必须通过任一非 Get 方法来处理事故，当事故未被处理时将会自动进一步升级
type AccidentSnapshot interface {
	// ActorContext 获取当前责任人上下文
	ActorContext() Context

	// GetPrimeCulprit 获取事故元凶，即导致事故的消息发送人
	GetPrimeCulprit() Ref

	// GetVictim 获取事故受害者，即接收到导致事故的消息的 Actor
	GetVictim() Ref

	// GetMessage 获取导致事故的消息
	GetMessage() core.Message

	// GetReason 获取事故原因
	GetReason() core.Message

	// GetStack 获取事件堆栈
	GetStack() []byte

	// GetRestartCount 获取重启次数
	GetRestartCount() int

	// Kill 立即停止目标 Actor 继续运行
	Kill(ref Ref, reason ...string)

	// PoisonKill 在目标 Actor 处理完剩余消息后停止其运行
	PoisonKill(ref Ref, reason ...string)

	// Resume 忽略本条消息并恢复事故受害者的运行
	Resume()

	// Restart 重启目标 Actor，并在重启后继续处理剩余消息
	Restart(ref Ref, reason ...string)

	// ExponentialBackoffRestart 退避指数重启
	ExponentialBackoffRestart(ref Ref, restartCount int, baseDelay, maxDelay time.Duration, multiplier, randomization float64, reason ...string)

	// Escalate 将事故升级至上级 Actor 处理
	Escalate()

	// SetResponsiblePerson 设置责任人
	SetResponsiblePerson(ctx Context)

	// IsFinished 是否已处理完毕
	IsFinished() bool

	// RecordRestartFailed 记录重启失败
	RecordRestartFailed(primeCulprit, victim Ref, message, reason core.Message, stack []byte)

	// IsDelayFinished 是否已处理完毕
	IsDelayFinished() bool
}
