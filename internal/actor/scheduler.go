package actor

import (
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/scheduler"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/robfig/cron/v3"
)

var (
	_ vivid.Scheduler     = (*Scheduler)(nil)
	_ vivid.ScheduledTask = (*scheduledTask)(nil)
)

func newScheduler(ctx *Context) *Scheduler {
	return &Scheduler{ctx: ctx, tasks: make(map[string]*scheduledTask)}
}

type Scheduler struct {
	ctx    *Context
	mu     sync.Mutex                // 协调登记、计时器与生命周期
	closed bool                      // 当前生命周期结束后永久关闭登记
	tasks  map[string]*scheduledTask // reference -> task
}

type scheduledTask struct {
	owner     atomic.Pointer[Scheduler] // 任务结束后解除对调度器及 Actor 上下文的引用
	reference string
	timer     *time.Timer
}

func (t *scheduledTask) Reference() string { return t.reference }

func (t *scheduledTask) Exists() bool {
	s := t.owner.Load()
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tasks[t.reference] == t
}

func (t *scheduledTask) Cancel() error {
	if s := t.owner.Load(); s != nil {
		return s.cancel(t.reference, t)
	}
	return vivid.ErrorNotFound.WithMessage(t.reference)
}

// release 在登记锁内释放底层引用，长期保存已结束句柄不再保留消息和 Actor 上下文。
func (t *scheduledTask) release() {
	t.timer = nil
	t.owner.Store(nil)
}

func (s *Scheduler) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clear()
}

func (s *Scheduler) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.clear()
}

// clear 在持有登记锁时清理任务，不改变生命周期准入。
func (s *Scheduler) clear() {
	for reference, task := range s.tasks {
		task.timer.Stop()
		delete(s.tasks, reference)
		task.release()
	}
}

// validate 在登记锁内复核生命周期与参数，不能调用用户提供的引用方法。
func (s *Scheduler) validate(receiver vivid.ActorRef) error {
	if s.ctx.system.requestsStopped() {
		return vivid.ErrorActorSystemStopped
	}
	if s.closed {
		return vivid.ErrorActorDeaded
	}
	if receiver == nil {
		return vivid.ErrorIllegalArgument.WithMessage("receiver is required")
	}
	value := reflect.ValueOf(receiver)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if value.IsNil() {
			return vivid.ErrorIllegalArgument.WithMessage("receiver is nil")
		}
	}
	return nil
}

func (s *Scheduler) register(reference string) (*scheduledTask, error) {
	if reference == "" {
		return nil, vivid.ErrorIllegalArgument.WithMessage("reference is required")
	}
	if _, exists := s.tasks[reference]; exists {
		return nil, vivid.ErrorScheduledTaskAlreadyExists.WithMessage(reference)
	}
	task := &scheduledTask{reference: reference}
	task.owner.Store(s)
	return task, nil
}

// deliver 沿用普通 Tell 的路由与框架消息语义，用户行为仍在目标邮箱中执行。
func (s *Scheduler) deliver(task *scheduledTask, receiver vivid.ActorRef, message vivid.Message) {
	defer func() {
		if fault := recover(); fault != nil {
			s.ctx.Logger().Error("scheduler delivery failed", log.String("reference", task.reference), log.Any("fault", fault))
		}
	}()
	s.ctx.Tell(receiver, message)
}

func (s *Scheduler) Exists(reference string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.tasks[reference]
	return ok
}

func (s *Scheduler) Cron(receiver vivid.ActorRef, expression string, message vivid.Message, options ...vivid.ScheduleOption) (vivid.ScheduledTask, error) {
	opts := vivid.NewScheduleOptions(options...)
	// 解析在登记锁外执行，结束后仍需在提交时复核生命周期。
	s.mu.Lock()
	err := s.validate(receiver)
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	plan, err := scheduler.ParseCron(expression, opts.Location)
	if err != nil {
		return nil, err
	}
	return s.scheduleCron(receiver, message, opts, plan)
}

// scheduleCron 将计时器与登记共同发布，即使立即到期也不会越过本次注册。
func (s *Scheduler) scheduleCron(receiver vivid.ActorRef, message vivid.Message, opts *vivid.ScheduleOptions, plan cron.Schedule) (vivid.ScheduledTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.validate(receiver); err != nil {
		return nil, err
	}
	task, err := s.register(opts.Reference)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	next := plan.Next(now)
	if !next.After(now) {
		task.release()
		return nil, vivid.ErrorScheduledTaskExpired
	}
	task.timer = time.AfterFunc(time.Until(next), func() { s.fireCron(task, receiver, message, plan, next) })
	s.tasks[task.reference] = task
	return task, nil
}

func (s *Scheduler) fireCron(task *scheduledTask, receiver vivid.ActorRef, message vivid.Message, plan cron.Schedule, at time.Time) {
	s.mu.Lock()
	if s.tasks[task.reference] != task {
		s.mu.Unlock()
		return
	}
	now := time.Now()
	if now.Before(at) {
		// 超过 time.Duration 上限的远期任务分段等待，不因时长饱和而提前投递。
		task.timer.Reset(at.Sub(now))
		s.mu.Unlock()
		return
	}
	next := plan.Next(now)
	if !next.After(now) {
		delete(s.tasks, task.reference)
		task.release()
	}
	s.mu.Unlock()
	s.deliver(task, receiver, message)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tasks[task.reference] != task {
		return
	}
	// 慢投递不补发经过的时间点，也不并发执行同一任务的投递。
	now = time.Now()
	if !next.After(now) {
		next = plan.Next(now)
	}
	if !next.After(now) {
		delete(s.tasks, task.reference)
		task.release()
		return
	}
	task.timer = time.AfterFunc(time.Until(next), func() { s.fireCron(task, receiver, message, plan, next) })
}

func (s *Scheduler) Once(receiver vivid.ActorRef, delay time.Duration, message vivid.Message, options ...vivid.ScheduleOption) (vivid.ScheduledTask, error) {
	return s.scheduleTimer(receiver, delay, false, message, vivid.NewScheduleOptions(options...))
}

func (s *Scheduler) Loop(receiver vivid.ActorRef, interval time.Duration, message vivid.Message, options ...vivid.ScheduleOption) (vivid.ScheduledTask, error) {
	return s.scheduleTimer(receiver, interval, true, message, vivid.NewScheduleOptions(options...))
}

func (s *Scheduler) scheduleTimer(receiver vivid.ActorRef, delay time.Duration, loop bool, message vivid.Message, opts *vivid.ScheduleOptions) (vivid.ScheduledTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.validate(receiver); err != nil {
		return nil, err
	}
	if loop && delay <= 0 {
		return nil, vivid.ErrorIllegalArgument.WithMessage("loop interval must be positive")
	}
	task, err := s.register(opts.Reference)
	if err != nil {
		return nil, err
	}
	task.timer = time.AfterFunc(delay, func() {
		if loop {
			s.fireLoop(task, receiver, message, delay)
		} else {
			s.fireOnce(task, receiver, message)
		}
	})
	s.tasks[task.reference] = task
	return task, nil
}

func (s *Scheduler) fireOnce(task *scheduledTask, receiver vivid.ActorRef, message vivid.Message) {
	s.mu.Lock()
	if s.tasks[task.reference] != task {
		s.mu.Unlock()
		return
	}
	delete(s.tasks, task.reference)
	task.release()
	s.mu.Unlock()
	s.deliver(task, receiver, message)
}

func (s *Scheduler) fireLoop(task *scheduledTask, receiver vivid.ActorRef, message vivid.Message, interval time.Duration) {
	s.mu.Lock()
	if s.tasks[task.reference] != task {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()
	s.deliver(task, receiver, message)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tasks[task.reference] == task {
		task.timer.Reset(interval)
	}
}

func (s *Scheduler) Cancel(reference string) error { return s.cancel(reference, nil) }

// cancel 在登记锁内核对句柄身份；按名称取消时 expected 为空。
func (s *Scheduler) cancel(reference string, expected *scheduledTask) error {
	s.mu.Lock()
	task, ok := s.tasks[reference]
	if !ok || expected != nil && task != expected {
		s.mu.Unlock()
		return vivid.ErrorNotFound.WithMessage(reference)
	}
	task.timer.Stop()
	delete(s.tasks, reference)
	task.release()
	s.mu.Unlock()
	// 用户日志可重入调度器，应在清理完成并释放登记锁后调用。
	s.ctx.Logger().Debug("scheduler cancel scheduled", log.String("ref", s.ctx.Ref().GetPath()), log.String("reference", reference))
	return nil
}
