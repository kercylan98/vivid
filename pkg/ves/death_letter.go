package ves

import (
	"time"

	"github.com/kercylan98/vivid"
)

// DeathLetterEvent 表示死信事件，用于记录无法被正常处理的消息。
//
// 当消息无法被 Actor 处理时（例如 Actor 已终止或处于非运行状态），
// 系统会将消息包装为 DeathLetterEvent 并发送到系统的守护 Actor（Guard Actor）。
// Guard Actor 会将该事件发布到事件流，以便订阅者进行监控和处理。
//
// 触发场景：
//   - Actor 已终止（killed 状态）时收到普通消息
//   - Actor 处于非运行状态（killing 状态）时收到普通消息
//   - 系统消息在 killing 阶段仍会被处理，不会成为死信
//
// 使用场景：
//   - 监控系统中无法处理的消息
//   - 诊断消息丢失或处理失败的问题
//   - 实现消息审计和追踪机制
//   - 收集系统异常情况下的消息统计
//
// 注意：
//   - 死信事件会被自动发送到系统的 Guard Actor
//   - Guard Actor 会将事件发布到事件流，可通过 EventStream().Subscribe() 订阅
//   - 建议在生产环境中监控死信事件，及时发现系统异常
type DeathLetterEvent struct {
	// Envelope 无法被处理的原始消息信封，包含发送者、接收者、消息内容等信息
	Envelope vivid.Envelop
	// Time 死信产生的时间戳，用于追踪消息处理的时间线
	Time time.Time
}
