package vivid

import (
	"time"

	"github.com/kercylan98/go-log/log"
)

var _ Actor = (*htwActor)(nil)

// HtwConfig 层级时间轮配置
type HtwConfig struct {
	Tick      time.Duration // 每 tick 的时间间隔
	WheelSize int64         // 时间轮大小
	Levels    int           // 层级数量
}

// DefaultHtwConfig 默认配置
func DefaultHtwConfig() *HtwConfig {
	return &HtwConfig{
		Tick:      time.Millisecond,
		WheelSize: 256,
		Levels:    4,
	}
}

func newHtwActor(config HtwConfig) *htwActor {
	h := &htwActor{
		tick:      int64(config.Tick / time.Millisecond),
		wheelSize: config.WheelSize,
		levels:    config.Levels,
		buckets:   make([][]*htwBucket, config.Levels),
		current:   make([]int64, config.Levels),
		interval:  make([]int64, config.Levels),
	}

	// 初始化每一层的时间轮
	for i := 0; i < config.Levels; i++ {
		h.buckets[i] = make([]*htwBucket, config.WheelSize)
		for j := int64(0); j < config.WheelSize; j++ {
			h.buckets[i][j] = newHtwBucket()
		}
		h.current[i] = 0
		if i == 0 {
			h.interval[i] = h.tick
		} else {
			h.interval[i] = h.interval[i-1] * h.wheelSize
		}
	}

	return h
}

// htwActor 是层级时间轮 Actor
// 它管理整个 ActorSystem 的定时器任务
type htwActor struct {
	tick      int64                 // 每 tick 毫秒
	wheelSize int64                 // 时间轮大小
	levels    int                   // 层级数量
	interval  []int64               // 每层的总跨度（毫秒）
	current   []int64               // 当前指针指向的位置
	buckets   [][]*htwBucket        // 每层的桶
	ticker    *time.Ticker          // 定时器
	stopChan  chan struct{}         // 停止信号
	index     map[string]*timerTask // 名称 -> 任务，支持覆盖/取消
}

// Receive 实现 Actor 接口
func (h *htwActor) Receive(context ActorContext) {
	switch m := context.Message().(type) {
	case *OnLaunch:
		h.onLaunch(context, m)
	case *OnKill:
		h.onKill(context, m)
	case *scheduleOnce:
		h.handleScheduleOnce(context, m)
	case *scheduleInterval:
		h.handleScheduleInterval(context, m)
	case *scheduleCron:
		h.handleScheduleCron(context, m)
	case *cancelSchedule:
		h.handleCancel(context, m)
	case *tickMsg:
		h.onTick(context, m)
	}
}

func (h *htwActor) onLaunch(context ActorContext, _ *OnLaunch) {
	// 启动定时器
	h.ticker = time.NewTicker(time.Duration(h.tick) * time.Millisecond)
	h.stopChan = make(chan struct{})
	if h.index == nil {
		h.index = make(map[string]*timerTask)
	}

	// 启动 tick 协程（仅发送消息，不直接修改状态）
	go h.tickLoop(context)

	context.Logger().Debug("time wheel started", "tick", h.tick, "wheelSize", h.wheelSize, "levels", h.levels)
}

func (h *htwActor) onKill(context ActorContext, _ *OnKill) {
	// 停止定时器
	if h.ticker != nil {
		h.ticker.Stop()
	}
	if h.stopChan != nil {
		close(h.stopChan)
	}

	context.Logger().Debug("time wheel stopped")
}

// tickLoop 定时器循环
func (h *htwActor) tickLoop(context ActorContext) {
	for {
		select {
		case <-h.ticker.C:
			context.Tell(context.Ref(), &tickMsg{})
		case <-h.stopChan:
			return
		}
	}
}

// onTick 处理时间轮推进（Actor 单线程消息处理，无需加锁）
func (h *htwActor) onTick(context ActorContext, _ *tickMsg) {
	// 推进第一层
	h.advanceLevel(context, 0)
}

// advanceLevel 推进指定层级
func (h *htwActor) advanceLevel(context ActorContext, level int) {
	if level >= h.levels {
		return
	}

	// 推进当前指针
	h.current[level] = (h.current[level] + 1) % h.wheelSize

	// 处理当前桶中的任务
	bucket := h.buckets[level][h.current[level]]
	tasks := bucket.flush()

	now := time.Now()
	nowMs := now.UnixMilli()
	for _, task := range tasks {
		if task == nil || task.canceled {
			continue
		}
		task.owner, task.element = nil, nil
		if task.isExpired(nowMs) {
			// 投递到目标 Actor：直接投递用户提供的负载
			if task.to != nil && task.payload != nil {
				context.Tell(task.to, task.payload)
			}
			// 周期/cron 计算下一次并重挂
			if task.computeNext(now) && !task.canceled {
				h.scheduleExistingTask(context, task)
				continue
			}
			// 一次性或无法计算下一次
			delete(h.index, task.name)
		} else {
			// 任务未到期，重新挂到合适层级
			h.scheduleExistingTask(context, task)
		}
	}

	// 如果当前层级指针回到0，推进下一层
	if h.current[level] == 0 && level+1 < h.levels {
		h.advanceLevel(context, level+1)
	}
}

// ---- 调度消息处理 ----
func (h *htwActor) handleScheduleOnce(ctx ActorContext, m *scheduleOnce) {
	h.cancelByName(m.Name)
	now := time.Now()
	task := &timerTask{
		name:     m.Name,
		mode:     timerModeOnce,
		expireAt: now.Add(m.Delay).UnixMilli(),
		to:       m.To,
		payload:  m.Payload,
	}
	h.index[m.Name] = task
	h.scheduleExistingTask(ctx, task)
}

func (h *htwActor) handleScheduleInterval(ctx ActorContext, m *scheduleInterval) {
	h.cancelByName(m.Name)
	now := time.Now()
	task := &timerTask{
		name:     m.Name,
		mode:     timerModeInterval,
		expireAt: now.Add(m.InitialDelay).UnixMilli(),
		to:       m.To,
		payload:  m.Payload,
		periodMs: m.Period.Milliseconds(),
	}
	h.index[m.Name] = task
	h.scheduleExistingTask(ctx, task)
}

func (h *htwActor) handleScheduleCron(ctx ActorContext, m *scheduleCron) {
	h.cancelByName(m.Name)
	sch, err := parseCronSpec(m.Spec)
	if err != nil {
		ctx.Logger().Error("invalid cron spec", log.Err(err), log.String("name", m.Name), log.String("spec", m.Spec))
		return
	}
	now := time.Now()
	next := sch.Next(now)
	if next.IsZero() {
		return
	}
	task := &timerTask{
		name:     m.Name,
		mode:     timerModeCron,
		expireAt: next.UnixMilli(),
		to:       m.To,
		payload:  m.Payload,
		cron:     sch,
	}
	h.index[m.Name] = task
	h.scheduleExistingTask(ctx, task)
}

func (h *htwActor) handleCancel(_ ActorContext, m *cancelSchedule) {
	h.cancelByName(m.Name)
}

func (h *htwActor) cancelByName(name string) {
	if task, ok := h.index[name]; ok && task != nil {
		task.canceled = true
		if task.owner != nil && task.element != nil {
			task.owner.removeElement(task.element)
		}
		delete(h.index, name)
	}
}

// scheduleTask 调度任务到合适的层级（Actor 单线程消息处理，无需加锁）
func (h *htwActor) scheduleExistingTask(_ ActorContext, task *timerTask) {
	nowMs := time.Now().UnixMilli()
	delay := task.expireAt - nowMs

	if delay <= 0 {
		delay = 0
	}

	// 选择层级：仅当 delay 覆盖当前层整轮跨度时才升级
	level := 0
	for level < h.levels-1 && delay >= h.interval[level]*h.wheelSize {
		level++
	}

	// 计算在指定层级中的位置
	ticks := delay / h.interval[level]
	if ticks == 0 {
		ticks = 1
	}
	index := (h.current[level] + ticks) % h.wheelSize

	// 添加到对应的桶中
	bucket := h.buckets[level][index]
	task.owner = bucket
	task.element = bucket.add(task)
}
