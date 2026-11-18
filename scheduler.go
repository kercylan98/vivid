package vivid

import (
	"time"

	"github.com/google/uuid"
)

// Scheduler 接口定义了由本地 Actor 生命周期管理的消息调度操作。
// 调度器允许对指定的 Actor 以多种调度策略（定时、延时、循环等）投递消息，
// 注册成功返回绑定本次任务的 ScheduledTask，也可通过引用标识管理任务。
// 引用标识仅在所属 Scheduler 内唯一，同路径重建的 Actor 使用独立的任务登记。
// Actor 终止或重启会清理任务；取消与清理不会撤回已经投递的消息。
// 收尾结束后旧 Scheduler 永久拒绝新任务，重启后需通过 ActorContext.Scheduler 获取新实例。
// Actor 已终止时注册返回 ErrorActorDeaded，系统停止完成后返回 ErrorActorSystemStopped。
// 同一 Scheduler 支持并发注册、查询、取消与清理。
// 投递沿用普通 Tell 的路由与框架消息语义，同一消息对象可被重复投递，应按不可变对象使用。
type Scheduler interface {
	// Cron 安排一个基于 Cron 表达式的周期性调度任务，定时向目标 Actor 投递消息。
	//
	// receiver: 目标 Actor 的引用（ActorRef），消息将被投递至该 Actor。
	// cron: 六段或七段表达式，秒、分、时、日、月、星期及可选年份（1970–9999）。
	// 星期为 0–6（周日至周六），日与星期不可同时限制；? 仅用于日或星期的通配。
	// 支持数字、名称、列表、范围与步长，不支持 L/W/#。如："0 0 0 * * ?" 表示每日 0 点。
	// message: 待调度投递的消息（Message），可以为任意实现了 Message 接口的对象。
	// options: 调度器可选配置（如 WithScheduleLocation、WithSchedulerReference）。
	//
	// 注册失败返回 nil 和具体错误，成功返回任务句柄和 nil。
	// 无未来触发时间返回 ErrorScheduledTaskExpired；表达式非法返回 ErrorCronParse。
	// 到期尽快投递一次，慢投递经过的时间点不补发；无后续时间时在末次投递前释放登记。
	Cron(receiver ActorRef, cron string, message Message, options ...ScheduleOption) (ScheduledTask, error)

	// Once 安排一个延时任务。在指定延时后仅执行一次，向目标 Actor 投递消息。
	//
	// receiver: 目标 Actor 的引用。
	// delay: 触发消息投递的延时时间（time.Duration），从当前时刻起算。
	// 非正延时尽快异步投递；取出投递前释放登记，接收方可复用 reference。
	// message: 投递的消息对象。
	// options: 调度任务可选参数配置。
	//
	// 注册失败返回 nil 和具体错误，成功返回任务句柄和 nil。
	// 非正延时的任务可能在本方法返回前已取出投递。
	Once(receiver ActorRef, delay time.Duration, message Message, options ...ScheduleOption) (ScheduledTask, error)

	// Loop 安排重复投递任务，首次等待 interval，之后在每次投递尝试结束后再等待 interval。
	// 同一任务的投递尝试不重叠；不等待目标 Actor 处理完成，不补发错过的周期。
	//
	// receiver: 目标 Actor 引用。
	// interval: 循环调度的时间间隔（time.Duration），必须大于零，否则返回 ErrorIllegalArgument。
	// message: 投递消息的内容。
	// options: 其他可选配置参数。
	//
	// 注册失败返回 nil 和具体错误，成功返回任务句柄和 nil。
	Loop(receiver ActorRef, interval time.Duration, message Message, options ...ScheduleOption) (ScheduledTask, error)

	// Exists 检查当前 Scheduler 是否保留指定引用标识(reference)的登记。
	// 一次任务取出投递时移除登记；false 不表示消息已处理，也不撤回在途投递。
	// Cron 无后续触发时间时移除登记。
	//
	// reference: 调度任务的唯一引用标识符（string）。
	//
	// 返回 bool：存在时为 true，否则为 false。
	Exists(reference string) bool

	// Cancel 取消当前使用该引用标识的任务；只取消某次注册应使用 ScheduledTask.Cancel。
	// 如果不存在对应引用，则返回 ErrorNotFound。
	// 一次任务已取出投递时返回 ErrorNotFound，不能阻止其继续投递。
	//
	// reference: 待取消的调度任务引用标识。
	//
	// 返回 error：取消成功返回 nil，失败返回原因。
	Cancel(reference string) error

	// Clear 清除当前调度器实例中所有已注册的调度任务。
	// 清理与注册串行完成，不禁止后续注册；并发注册的任务可能在清理后加入。
	Clear()
}

// ScheduledTask 绑定一次成功注册，不会因 reference 复用或 Actor 重启指向新任务。
// 支持并发查询和取消；不表示目标 Actor 已收到或处理消息。
type ScheduledTask interface {
	// Reference 返回注册时的引用标识，未显式指定时由框架生成。
	Reference() string
	// Exists 检查本次任务是否仍保留登记，不查询同名的其他任务。
	// Once 取出投递、取消或清理后返回 false；Cron 无后续触发时间时释放登记。
	Exists() bool
	// Cancel 只取消本次任务；登记已释放时返回 ErrorNotFound。
	// 不能撤回已取出投递或已进入邮箱的消息。
	Cancel() error
}

// NewScheduleOptions 应用配置后补齐默认时区与引用标识，nil 配置项忽略。
func NewScheduleOptions(options ...ScheduleOption) *ScheduleOptions {
	opts := &ScheduleOptions{
		Location: time.Local,
	}
	for _, option := range options {
		if option != nil {
			option(opts)
		}
	}
	if opts.Location == nil {
		opts.Location = time.Local
	}
	if opts.Reference == "" {
		opts.Reference = uuid.New().String()
	}
	return opts
}

type ScheduleOptions struct {
	Location  *time.Location // 调度器的位置，默认为 time.Local，用于 cron 表达式的时间计算
	Reference string         // 调度任务引用标识，用于 Exists/Cancel；不传时由 NewScheduleOptions 自动生成
}

type ScheduleOption func(options *ScheduleOptions)

// WithScheduleOptions 整体设置调度器选项，空字段在配置应用结束后使用默认值。
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

// WithSchedulerReference 设置调度器引用标识，它可被用于取消调度等操作
func WithSchedulerReference(reference string) ScheduleOption {
	return func(opts *ScheduleOptions) {
		if reference == "" {
			return
		}
		opts.Reference = reference
	}
}
