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
	_                  vivid.Scheduler = (*Scheduler)(nil)
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
		jobKeys:   make(map[string]*quartz.JobKey),
	}
}

type Scheduler struct {
	ctx       *Context
	scheduler *scheduler.Scheduler
	mu        sync.Mutex                // 协调本地登记与底层任务的注册和清理
	namespace string                    // 实例独立的任务分组，不依赖可复用的 Actor 路径
	jobKeys   map[string]*quartz.JobKey // reference -> job key
}

func (s *Scheduler) jobKey(reference string) *quartz.JobKey {
	return quartz.NewJobKeyWithGroup(reference, s.namespace)
}

func (s *Scheduler) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for reference, jobKey := range s.jobKeys {
		_ = s.scheduler.DeleteJob(jobKey)
		delete(s.jobKeys, reference)
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

func (s *Scheduler) tell(receiver vivid.ActorRef, message vivid.Message, options *vivid.ScheduleOptions) {
	schedulerMessage := &SchedulerMessage{
		Reference: options.Reference,
		Message:   message,
	}
	//s.ctx.Logger().Debug("scheduler trigger", log.String("reference", options.Reference), log.Time("time", time.Now()), log.String("messageType", fmt.Sprintf("%T", message)))
	if receiver.Equals(s.ctx.Ref()) {
		s.ctx.TellSelf(schedulerMessage)
		return
	}
	s.ctx.Tell(receiver, schedulerMessage)
}

// scheduleJob 注册任务并调度，将 reference 与 jobKey 记入 jobKeys 供 Exists/Cancel 使用
func (s *Scheduler) scheduleJob(receiver vivid.ActorRef, message vivid.Message, opts *vivid.ScheduleOptions, trigger quartz.Trigger, logKind string, logFields ...any) error {
	jobKey := s.jobKey(opts.Reference)
	fn := job.NewFunctionJob(func(ctx context.Context) (any, error) {
		s.tell(receiver, message, opts)
		return nil, nil
	})
	// 本地登记与底层注册共同提交，清理不能越过尚未完成的注册。
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.scheduler.Schedule(quartz.NewJobDetail(fn, jobKey), trigger); err != nil {
		return schedulerErrorConvert(err)
	}
	s.jobKeys[opts.Reference] = jobKey

	// base := []any{log.String("ref", s.ctx.Ref().GetPath()), log.String("receiver", receiver.GetPath()), log.String("messageType", fmt.Sprintf("%T", message))}
	// logKindStr := fmt.Sprintf("scheduler %s scheduled", logKind)
	// s.ctx.Logger().Debug(logKindStr, append(base, logFields...)...)
	return nil
}

func (s *Scheduler) Exists(reference string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.jobKeys[reference]
	return ok
}

func (s *Scheduler) Cron(receiver vivid.ActorRef, cron string, message vivid.Message, options ...vivid.ScheduleOption) error {
	opts := vivid.NewScheduleOptions(options...)
	trigger, err := quartz.NewCronTriggerWithLoc(cron, opts.Location)
	if err = schedulerErrorConvert(err); err != nil {
		return err
	}
	return s.scheduleJob(receiver, message, opts, trigger, "cron", log.String("cron", cron))
}

func (s *Scheduler) Once(receiver vivid.ActorRef, delay time.Duration, message vivid.Message, options ...vivid.ScheduleOption) error {
	opts := vivid.NewScheduleOptions(options...)
	return s.scheduleJob(receiver, message, opts, quartz.NewRunOnceTrigger(delay), "once", log.Duration("delay", delay))
}

func (s *Scheduler) Loop(receiver vivid.ActorRef, interval time.Duration, message vivid.Message, options ...vivid.ScheduleOption) error {
	opts := vivid.NewScheduleOptions(options...)
	return s.scheduleJob(receiver, message, opts, quartz.NewSimpleTrigger(interval), "loop", log.Duration("interval", interval))
}

func (s *Scheduler) Cancel(reference string) error {
	s.mu.Lock()
	jobKey, ok := s.jobKeys[reference]
	if !ok {
		s.mu.Unlock()
		return vivid.ErrorNotFound.WithMessage(reference)
	}
	err := s.scheduler.DeleteJob(jobKey)
	delete(s.jobKeys, reference)
	s.mu.Unlock()
	// 用户日志可重入调度器，应在底层清理完成并释放登记锁后调用。
	s.ctx.Logger().Debug("scheduler cancel scheduled", log.String("ref", s.ctx.Ref().GetPath()), log.String("reference", reference))
	return schedulerErrorConvert(err)
}
