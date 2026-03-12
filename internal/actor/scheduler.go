package actor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/scheduler"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/reugn/go-quartz/job"
	"github.com/reugn/go-quartz/quartz"
)

// uniqueJobKey 使用 Context.instanceID 保证按 Actor 实例隔离，避免跨 ActorSystem/同 Ref 时同一 reference 导致任务互相覆盖。
func uniqueJobKey(ctx *Context, reference string) *quartz.JobKey {
	jobKey := fmt.Sprintf("%s:%s", ctx.ref.GetPath(), reference)
	return quartz.NewJobKey(jobKey)
}

var (
	_ vivid.Scheduler = (*Scheduler)(nil)
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
		jobKeys:   make(map[string]*quartz.JobKey),
	}
}

type Scheduler struct {
	ctx       *Context
	scheduler *scheduler.Scheduler
	jobKeys   map[string]*quartz.JobKey // actor reference -> job key
}

func (s *Scheduler) Clear() {
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
	jobKey := uniqueJobKey(s.ctx, opts.Reference)
	s.jobKeys[opts.Reference] = jobKey
	fn := job.NewFunctionJob(func(ctx context.Context) (any, error) {
		s.tell(receiver, message, opts)
		return nil, nil
	})
	if err := s.scheduler.Schedule(quartz.NewJobDetail(fn, jobKey), trigger); err != nil {
		return schedulerErrorConvert(err)
	}

	base := []any{log.String("ref", s.ctx.Ref().GetPath()), log.String("receiver", receiver.GetPath()), log.String("messageType", fmt.Sprintf("%T", message))}
	logKindStr := fmt.Sprintf("scheduler %s scheduled", logKind)
	s.ctx.Logger().Debug(logKindStr, append(base, logFields...)...)
	return nil
}

func (s *Scheduler) Exists(reference string) bool {
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
	jobKey, ok := s.jobKeys[reference]
	if !ok {
		return vivid.ErrorNotFound.WithMessage(reference)
	}
	delete(s.jobKeys, reference)
	s.ctx.Logger().Debug("scheduler cancel scheduled", log.String("ref", s.ctx.Ref().GetPath()), log.String("reference", reference))
	return schedulerErrorConvert(s.scheduler.DeleteJob(jobKey))
}
