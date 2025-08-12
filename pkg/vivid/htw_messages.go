package vivid

import (
	"container/list"
	"time"
)

// 内部时间轮消息（不导出）
type tickMsg struct{}

// 调度模式
type timerMode int

const (
	timerModeOnce timerMode = iota
	timerModeInterval
	timerModeCron
)

// 内部调度消息（不导出）
type scheduleOnce struct {
	Name    string
	Delay   time.Duration
	To      ActorRef
	Payload Message
}

type scheduleInterval struct {
	Name         string
	InitialDelay time.Duration
	Period       time.Duration
	To           ActorRef
	Payload      Message
}

type scheduleCron struct {
	Name    string
	Spec    string // Cron 表达式
	To      ActorRef
	Payload Message
}

type cancelSchedule struct{ Name string }

// timerTask 定时器任务（内部）
type timerTask struct {
	name     string
	mode     timerMode
	expireAt int64 // 毫秒时间戳
	to       ActorRef
	payload  Message
	// interval
	periodMs int64
	// cron
	cron cronSchedule
	// 状态
	canceled bool
	// 结构位置
	owner   *htwBucket
	element *list.Element
}

func (t *timerTask) isExpired(nowMs int64) bool { return nowMs >= t.expireAt }

// 计算下次触发时间，返回是否应继续（false 表示一次性/已无下次）
func (t *timerTask) computeNext(now time.Time) bool {
	switch t.mode {
	case timerModeOnce:
		return false
	case timerModeInterval:
		// 固定频率：下一次 = 上次计划时间 + period
		t.expireAt += t.periodMs
		return true
	case timerModeCron:
		if t.cron == nil {
			return false
		}
		next := t.cron.Next(now)
		if next.IsZero() {
			return false
		}
		t.expireAt = next.UnixMilli()
		return true
	default:
		return false
	}
}

// cronSchedule 定义了 Cron 的 Next 计算
type cronSchedule interface{ Next(time.Time) time.Time }

// 解析 Cron 表达式（保持内部实现可替换、对外不暴露）
func parseCronSpec(spec string) (cronSchedule, error) {
	// 简易内置实现：按标准 6/7 字段格式秒分时日月周年，依赖 robfig/cron 是常规做法
	// 但为保持内部实现不外露，我们在此处延迟注入，默认返回错误
	return parseCronSpecRobfig(spec)
}
