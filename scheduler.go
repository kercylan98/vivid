package vivid

import (
	"time"

	"github.com/google/uuid"
)

// Scheduler 接口定义了分布式 Actor 调度系统中所有调度相关的核心操作。
// 调度器允许对指定的 Actor 以多种调度策略（定时、延时、循环等）投递消息，
// 并支持通过引用标识管理、取消及查询调度任务的状态。
// 实现类需保证线程安全与高效的任务调度能力。
type Scheduler interface {
	// Cron 安排一个基于 Cron 表达式的周期性调度任务，定时向目标 Actor 投递消息。
	//
	// receiver: 目标 Actor 的引用（ActorRef），消息将被投递至该 Actor。
	// cron: Cron 表达式字符串，定义任务的运行时间计划。如："0 0 * * *" 表示每日 0 点。
	// message: 待调度投递的消息（Message），可以为任意实现了 Message 接口的对象。
	// options: 调度器可选配置（如 Location, Reference），可通过 WithScheduleXXX 方法链式传递。
	//
	// 返回 error：若调度安排失败，返回具体错误，否则为 nil。
	Cron(receiver ActorRef, cron string, message Message, options ...ScheduleOption) error

	// Once 安排一个延时任务。在指定延时后仅执行一次，向目标 Actor 投递消息。
	//
	// receiver: 目标 Actor 的引用。
	// delay: 触发消息投递的延时时间（time.Duration），从当前时刻起算。
	// message: 投递的消息对象。
	// options: 调度任务可选参数配置。
	//
	// 返回 error：调度失败时返回具体错误，否则为 nil。
	Once(receiver ActorRef, delay time.Duration, message Message, options ...ScheduleOption) error

	// Loop 安排一个固定间隔重复执行的调度任务，每间隔指定时间向目标 Actor 投递消息。
	//
	// receiver: 目标 Actor 引用。
	// interval: 循环调度的时间间隔（time.Duration）。
	// message: 投递消息的内容。
	// options: 其他可选配置参数。
	//
	// 返回 error：若调度失败返回具体错误，否则为 nil。
	Loop(receiver ActorRef, interval time.Duration, message Message, options ...ScheduleOption) error

	// Exists 检查指定引用标识(reference)的调度任务是否存在（未被取消或尚未到期）。
	//
	// reference: 调度任务的唯一引用标识符（string）。
	//
	// 返回 bool：存在时为 true，否则为 false。
	Exists(reference string) bool

	// Cancel 通过引用标识取消指定的调度任务（无论其当前状态）。如果不存在对应引用，则返回错误。
	//
	// reference: 待取消的调度任务引用标识。
	//
	// 返回 error：取消成功返回 nil，失败返回原因。
	Cancel(reference string) error

	// Clear 清除当前调度器实例中所有已注册的调度任务。
	Clear()
}

func NewScheduleOptions(options ...ScheduleOption) *ScheduleOptions {
	opts := &ScheduleOptions{
		Location:  time.Local,
		Reference: uuid.New().String(),
	}
	for _, option := range options {
		option(opts)
	}
	return opts
}

type ScheduleOptions struct {
	Location  *time.Location // 调度器的位置，默认为 time.Local，用于 cron 表达式的时间计算
	Reference string         // 调度器引用标识
}

type ScheduleOption func(options *ScheduleOptions)

// WithScheduleOptions 设置调度器选项
func WithScheduleOptions(options ScheduleOptions) ScheduleOption {
	return func(opts *ScheduleOptions) {
		*opts = options
	}
}

// WithScheduleLocation 设置调度器的位置，默认为 time.Local
func WithScheduleLocation(location *time.Location) ScheduleOption {
	return func(opts *ScheduleOptions) {
		if location == nil {
			return
		}
		opts.Location = location
	}
}

// WithReference 设置调度器引用标识，它可被用于取消调度等操作
func WithReference(reference string) ScheduleOption {
	return func(opts *ScheduleOptions) {
		if reference == "" {
			return
		}
		opts.Reference = reference
	}
}
