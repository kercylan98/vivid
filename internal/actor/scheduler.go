package actor

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/scheduler"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/reugn/go-quartz/job"
	"github.com/reugn/go-quartz/quartz"
)

var (
	_                  vivid.Scheduler     = (*Scheduler)(nil)
	_                  vivid.ScheduledTask = (*scheduledTask)(nil)
	schedulerNamespace atomic.Uint64
)

// quartzErrorToVivid 将 Quartz 错误转换为 Vivid 错误
var quartzErrorToVivid = []struct {
	match func(error) bool
	vivid *vivid.Error
}{
	{func(e error) bool { return errors.Is(e, quartz.ErrCronParse) }, vivid.ErrorCronParse},
	//{func(e error) bool { return errors.Is(e, quartz.ErrTriggerExpired) }, vivid.ErrorTriggerExpired},
	//{func(e error) bool { return errors.Is(e, quartz.ErrIllegalState) }, vivid.ErrorIllegalState},
	//{func(e error) bool { return errors.Is(e, quartz.ErrQueueEmpty) }, vivid.ErrorQueueEmpty},
	//{func(e error) bool { return errors.Is(e, quartz.ErrJobAlreadyExists) }, vivid.ErrorJobAlreadyExists},
	//{func(e error) bool { return errors.Is(e, quartz.ErrJobIsSuspended) }, vivid.ErrorJobIsSuspended},
	//{func(e error) bool { return errors.Is(e, quartz.ErrJobIsActive) }, vivid.ErrorJobIsActive},
}

type SchedulerMessage struct {
	Reference string
	Message   vivid.Message
}

func newScheduler(ctx *Context) *Scheduler {
	return &Scheduler{
		ctx:       ctx,
		scheduler: ctx.system.scheduler,
		namespace: strconv.FormatUint(schedulerNamespace.Add(1), 10),
		tasks:     make(map[string]*scheduledTask),
	}
}

type Scheduler struct {
	ctx       *Context
	scheduler *scheduler.Scheduler
	mu        sync.Mutex                // 协调本地登记与底层任务的注册和清理
	closed    bool                      // 当前生命周期结束后永久关闭登记
	namespace string                    // 实例独立的任务分组，不依赖可复用的 Actor 路径
	tasks     map[string]*scheduledTask // reference -> task
}

type scheduledTask struct {
	owner     atomic.Pointer[Scheduler] // 任务结束后解除对调度器及 Actor 上下文的引用
	reference string
	jobKey    *quartz.JobKey
	timer     *time.Timer
}

type schedulerCronTrigger struct {
	quartz.Trigger
	task    *scheduledTask
	started bool // 首次计算属于注册，发布后由 Quartz 在队列锁内串行计算
}

func (t *schedulerCronTrigger) NextFireTime(prev int64) (int64, error) {
	next, err := t.Trigger.NextFireTime(prev)
	if !t.started {
		t.started = true
		return next, err
	}
	if err != nil {
		if s := t.task.owner.Load(); s != nil {
			// Quartz 持有队列锁，不能同步取得登记锁；错过末次投递也需回收。
			go s.expireTask(t.task)
		}
	}
	return next, err
}

func (s *Scheduler) expireTask(task *scheduledTask) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tasks[task.reference] != task {
		return
	}
	// 底层已取出且不再入队，只回收本次登记，不能按名称删除新任务。
	delete(s.tasks, task.reference)
	task.release()
}

func (t *scheduledTask) Reference() string {
	return t.reference
}

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
	t.jobKey = nil
	t.owner.Store(nil)
}

func (s *Scheduler) cancelTask(task *scheduledTask) error {
	if task.timer != nil {
		task.timer.Stop()
		return nil
	}
	err := s.scheduler.DeleteJob(task.jobKey)
	// Cron 可能已从底层队列取出，但自然结束的回收尚未取得登记锁。
	if errors.Is(err, quartz.ErrJobNotFound) {
		return nil
	}
	return err
}

func (s *Scheduler) jobKey(reference string) *quartz.JobKey {
	return quartz.NewJobKeyWithGroup(reference, s.namespace)
}

func (s *Scheduler) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clear()
}

// close 与登记共用锁，已进入底层注册的任务先提交，再随本次关闭清理。
func (s *Scheduler) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.clear()
}

// clear 在持有登记锁时清理任务，不改变生命周期准入。
func (s *Scheduler) clear() {
	for reference, task := range s.tasks {
		_ = s.cancelTask(task)
		delete(s.tasks, reference)
		task.release()
	}
}

func schedulerErrorConvert(err error) error {
	if err == nil {
		return nil
	}
	for _, m := range quartzErrorToVivid {
		if m.match(err) {
			err = m.vivid.With(err)
			break
		}
	}
	return err
}

func (s *Scheduler) tell(receiver vivid.ActorRef, message vivid.Message, reference string) {
	// 远程只传业务消息，任务登记归本地所有，不传递无法独立解码的接口包装。
	if receiver.GetAddress() != s.ctx.Ref().GetAddress() {
		s.ctx.Tell(receiver, message)
		return
	}
	schedulerMessage := &SchedulerMessage{
		Reference: reference,
		Message:   message,
	}
	//s.ctx.Logger().Debug("scheduler trigger", log.String("reference", options.Reference), log.Time("time", time.Now()), log.String("messageType", fmt.Sprintf("%T", message)))
	if receiver.Equals(s.ctx.Ref()) {
		s.ctx.TellSelf(schedulerMessage)
		return
	}
	s.ctx.Tell(receiver, schedulerMessage)
}

// scheduleJob 注册 Quartz 任务并保存登记，供 Exists/Cancel 使用。
func (s *Scheduler) scheduleJob(receiver vivid.ActorRef, message vivid.Message, opts *vivid.ScheduleOptions, trigger quartz.Trigger, logKind string, logFields ...any) (vivid.ScheduledTask, error) {
	jobKey := s.jobKey(opts.Reference)
	reference := opts.Reference
	fn := job.NewFunctionJob(func(ctx context.Context) (any, error) {
		s.tell(receiver, message, reference)
		return nil, nil
	})
	// 本地登记与底层注册共同提交，清理不能越过尚未完成的注册。
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.system.requestsStopped() {
		return nil, vivid.ErrorActorSystemStopped
	}
	if s.closed {
		return nil, vivid.ErrorActorDeaded
	}
	if receiver == nil || reference == "" {
		return nil, vivid.ErrorIllegalArgument.WithMessage("receiver and reference are required")
	}
	// 非正周期会不断产生已经到期的任务，必须在进入底层队列前拒绝。
	if loop, ok := trigger.(*quartz.SimpleTrigger); ok && loop.Interval <= 0 {
		return nil, vivid.ErrorIllegalArgument.WithMessage("loop interval must be positive")
	}
	if _, exists := s.tasks[reference]; exists {
		return nil, quartz.ErrJobAlreadyExists
	}
	task := &scheduledTask{reference: reference, jobKey: jobKey}
	task.owner.Store(s)
	if cron, ok := trigger.(*schedulerCronTrigger); ok {
		cron.task = task
	}
	if err := s.scheduler.Schedule(quartz.NewJobDetail(fn, jobKey), trigger); err != nil {
		task.release()
		return nil, schedulerErrorConvert(err)
	}
	s.tasks[reference] = task

	// base := []any{log.String("ref", s.ctx.Ref().GetPath()), log.String("receiver", receiver.GetPath()), log.String("messageType", fmt.Sprintf("%T", message))}
	// logKindStr := fmt.Sprintf("scheduler %s scheduled", logKind)
	// s.ctx.Logger().Debug(logKindStr, append(base, logFields...)...)
	return task, nil
}

func (s *Scheduler) Exists(reference string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.tasks[reference]
	return ok
}

func (s *Scheduler) Cron(receiver vivid.ActorRef, cron string, message vivid.Message, options ...vivid.ScheduleOption) (vivid.ScheduledTask, error) {
	opts := vivid.NewScheduleOptions(options...)
	trigger, err := quartz.NewCronTriggerWithLoc(cron, opts.Location)
	if err = schedulerErrorConvert(err); err != nil {
		return nil, err
	}
	return s.scheduleJob(receiver, message, opts, &schedulerCronTrigger{Trigger: trigger}, "cron", log.String("cron", cron))
}

func (s *Scheduler) Once(receiver vivid.ActorRef, delay time.Duration, message vivid.Message, options ...vivid.ScheduleOption) (vivid.ScheduledTask, error) {
	opts := vivid.NewScheduleOptions(options...)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.system.requestsStopped() {
		return nil, vivid.ErrorActorSystemStopped
	}
	if s.closed {
		return nil, vivid.ErrorActorDeaded
	}
	if receiver == nil || opts.Reference == "" {
		return nil, vivid.ErrorIllegalArgument.WithMessage("receiver and reference are required")
	}
	if _, exists := s.tasks[opts.Reference]; exists {
		return nil, quartz.ErrJobAlreadyExists
	}
	// 回调与注册共用锁，即使立即到期也要等到 timer 与登记一同发布。
	task := &scheduledTask{reference: opts.Reference}
	task.owner.Store(s)
	task.timer = time.AfterFunc(delay, func() { s.fireOnce(task, receiver, message) })
	s.tasks[task.reference] = task
	return task, nil
}

func (s *Scheduler) fireOnce(task *scheduledTask, receiver vivid.ActorRef, message vivid.Message) {
	s.mu.Lock()
	if s.tasks[task.reference] != task {
		s.mu.Unlock()
		return
	}
	// 先移除本次登记再投递，接收方可以立即复用 reference。
	delete(s.tasks, task.reference)
	task.release()
	s.mu.Unlock()
	// 投递可能经过用户提供的引用或邮箱，保留原 Quartz 路径的 panic 隔离。
	defer func() {
		if fault := recover(); fault != nil {
			s.ctx.Logger().Error("scheduler once delivery failed", log.String("reference", task.reference), log.Any("fault", fault))
		}
	}()
	s.tell(receiver, message, task.reference)
}

func (s *Scheduler) Loop(receiver vivid.ActorRef, interval time.Duration, message vivid.Message, options ...vivid.ScheduleOption) (vivid.ScheduledTask, error) {
	opts := vivid.NewScheduleOptions(options...)
	return s.scheduleJob(receiver, message, opts, quartz.NewSimpleTrigger(interval), "loop", log.Duration("interval", interval))
}

func (s *Scheduler) Cancel(reference string) error {
	return s.cancel(reference, nil)
}

// cancel 在登记锁内核对句柄身份；按名称取消时 expected 为空。
func (s *Scheduler) cancel(reference string, expected *scheduledTask) error {
	s.mu.Lock()
	task, ok := s.tasks[reference]
	if !ok || expected != nil && task != expected {
		s.mu.Unlock()
		return vivid.ErrorNotFound.WithMessage(reference)
	}
	err := s.cancelTask(task)
	delete(s.tasks, reference)
	task.release()
	s.mu.Unlock()
	// 用户日志可重入调度器，应在底层清理完成并释放登记锁后调用。
	s.ctx.Logger().Debug("scheduler cancel scheduled", log.String("ref", s.ctx.Ref().GetPath()), log.String("reference", reference))
	return schedulerErrorConvert(err)
}
