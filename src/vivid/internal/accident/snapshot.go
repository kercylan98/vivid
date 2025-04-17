package accident

import (
	"github.com/kercylan98/vivid/src/vivid/internal/core"
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
	"github.com/kercylan98/vivid/src/vivid/internal/core/mailbox"
	"math"
	"math/rand/v2"
	"strings"
	"sync/atomic"
	"time"
)

var _ actor.Snapshot = (*Snapshot)(nil)

func NewSnapshot(mailbox mailbox.Mailbox, primeCulprit, victim actor.Ref, message, reason core.Message, stack []byte) *Snapshot {
	return &Snapshot{
		Mailbox:      mailbox,
		PrimeCulprit: primeCulprit,
		Victim:       victim,
		Message:      message,
		Reason:       reason,
		Stack:        stack,
	}
}

type Snapshot struct {
	Mailbox           mailbox.Mailbox // 事故受害者的邮箱
	responsiblePerson actor.Context   // 当前责任人上下文
	PrimeCulprit      actor.Ref       // 事故元凶
	Victim            actor.Ref       // 事故受害者
	Message           core.Message    // 造成事故发生的消息
	Reason            core.Message    // 事故原因
	Stack             []byte          // 事件堆栈
	RestartTimes      []time.Time     // 重启时间
	Finished          atomic.Bool     // 是否已经处理完毕
	DelayFinished     bool            // 是否是延迟处理的（退避指数重启）
}

func (s *Snapshot) ActorContext() actor.Context {
	return s.responsiblePerson
}

func (s *Snapshot) GetPrimeCulprit() actor.Ref {
	return s.PrimeCulprit
}

func (s *Snapshot) GetVictim() actor.Ref {
	return s.Victim
}

func (s *Snapshot) GetMessage() core.Message {
	return s.Message
}

func (s *Snapshot) GetReason() core.Message {
	return s.Reason
}

func (s *Snapshot) GetStack() []byte {
	return s.Stack
}

func (s *Snapshot) GetRestartCount() int {
	return len(s.RestartTimes)
}

func (s *Snapshot) Kill(ref actor.Ref, reason ...string) {
	if !s.Finished.CompareAndSwap(false, true) {
		return
	}
	s.responsiblePerson.TransportContext().Tell(ref, core.SystemMessage, &actor.OnKill{
		Reason:   strings.Join(reason, ", "),
		Operator: s.responsiblePerson.MetadataContext().Ref(),
	})
}

func (s *Snapshot) PoisonKill(ref actor.Ref, reason ...string) {
	if !s.Finished.CompareAndSwap(false, true) {
		return
	}
	s.Mailbox.Resume()
	s.responsiblePerson.TransportContext().Tell(ref, core.UserMessage, &actor.OnKill{
		Reason:   strings.Join(reason, ", "),
		Operator: s.responsiblePerson.MetadataContext().Ref(),
		Poison:   true,
	})
}

func (s *Snapshot) Resume() {
	if !s.Finished.CompareAndSwap(false, true) {
		return
	}
	s.Mailbox.Resume()
}

func (s *Snapshot) Restart(ref actor.Ref, reason ...string) {
	if !s.Finished.CompareAndSwap(false, true) {
		return
	}
	s.Mailbox.Resume()

	// 当意外发生后，Actor 的状态无法得到保证，需要在重启后继续处理剩余消息，所以不需要优雅重启
	s.responsiblePerson.TransportContext().Tell(ref, core.SystemMessage, &actor.OnKill{
		Reason:   strings.Join(reason, ", "),
		Operator: s.responsiblePerson.MetadataContext().Ref(),
		Restart:  true,
	})
}

func (s *Snapshot) ExponentialBackoffRestart(ref actor.Ref, restartCount int, baseDelay, maxDelay time.Duration, multiplier, randomization float64, reason ...string) {
	if !s.Finished.CompareAndSwap(false, true) {
		return
	}
	s.DelayFinished = true
	// 如果是重启，通过退避策略来控制重启次数，达到上限后停止。
	delay := float64(baseDelay) * math.Pow(multiplier, float64(s.GetRestartCount()))
	jitter := (rand.Float64() - 0.5) * randomization * float64(baseDelay)
	after := time.Duration(delay + jitter)
	if after > maxDelay {
		after = maxDelay
	}

	if count := s.GetRestartCount(); count >= restartCount {
		s.Finished.Store(false)
		s.Kill(ref, "supervisor: OnLaunch restart fail count limit")

		s.responsiblePerson.TransportContext().Tell(ref, core.SystemMessage, actor.SnapshotEnd{Snapshot: s})
	} else {
		// 使用当前责任人的定时器来执行重启操作
		time.AfterFunc(after, func() {
			s.Finished.Store(false)
			s.Restart(ref, reason...)
			s.responsiblePerson.TransportContext().Tell(ref, core.SystemMessage, actor.SnapshotEnd{Snapshot: s})
		})
	}
}

func (s *Snapshot) Escalate() {
	if !s.Finished.CompareAndSwap(false, true) {
		return
	}
	s.responsiblePerson.TransportContext().Tell(s.responsiblePerson.MetadataContext().Parent(), core.SystemMessage, s)
}

func (s *Snapshot) SetResponsiblePerson(ctx actor.Context) {
	s.responsiblePerson = ctx
	s.Finished.Store(false)
	s.DelayFinished = false
}

func (s *Snapshot) IsFinished() bool {
	return s.Finished.Load()
}
func (s *Snapshot) IsDelayFinished() bool {
	return s.DelayFinished
}

func (s *Snapshot) RecordRestartFailed(primeCulprit, victim actor.Ref, message, reason core.Message, stack []byte) {
	s.RestartTimes = append(s.RestartTimes, time.Now())
	s.PrimeCulprit = primeCulprit
	s.Victim = victim
	s.Message = message
	s.Reason = reason
	s.Stack = stack
}
