package actor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/messages"
	"github.com/kercylan98/vivid/internal/scheduler"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/reugn/go-quartz/job"
	"github.com/reugn/go-quartz/quartz"
)

var (
	_ vivid.Scheduler = (*Scheduler)(nil)
)

func init() {
	messages.RegisterInternalMessage[*SchedulerMessage]("SchedulerMessage", schedulerMessageReader, schedulerMessageWriter)
}

type SchedulerMessage struct {
	Reference string
	Message   vivid.Message
	Once      bool
}

func schedulerMessageReader(message any, reader *messages.Reader, codec messages.Codec) (err error) {
	m := message.(*SchedulerMessage)

	if m.Message, err = reader.ReadMessage(codec); err != nil {
		return err
	}

	return reader.ReadInto(&m.Reference, &m.Once)
}

func schedulerMessageWriter(message any, writer *messages.Writer, codec messages.Codec) (err error) {
	m := message.(*SchedulerMessage)

	if err = writer.WriteMessage(m.Message, codec); err != nil {
		return err
	}

	return writer.WriteFrom(m.Reference, m.Once)
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
	jobKeys   map[string]*quartz.JobKey // reference -> job key
}

func (s *Scheduler) Clear() {
	for reference, jobKey := range s.jobKeys {
		if err := s.scheduler.DeleteJob(jobKey); err != nil {
			switch {
			case !errors.Is(err, quartz.ErrJobNotFound):
				s.ctx.Logger().Warn("failed to delete job", log.String("reference", reference), log.Any("error", err))
			}
		}
		delete(s.jobKeys, reference)
	}
}

func uniqueJobKey(ctx vivid.ActorContext, reference string) *quartz.JobKey {
	jobKey := ctx.Ref().GetPath() + ":" + reference
	return quartz.NewJobKey(jobKey)
}

func schedulerErrorConvert(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, quartz.ErrJobNotFound):
		return vivid.ErrorNotFound.WithMessage(err.Error())
	case errors.Is(err, quartz.ErrCronParse):
		return vivid.ErrorCronParse.WithMessage(err.Error())
	case errors.Is(err, quartz.ErrTriggerExpired):
		return vivid.ErrorTriggerExpired.WithMessage(err.Error())
	case errors.Is(err, quartz.ErrIllegalState):
		return vivid.ErrorIllegalState.WithMessage(err.Error())
	case errors.Is(err, quartz.ErrQueueEmpty):
		return vivid.ErrorQueueEmpty.WithMessage(err.Error())
	case errors.Is(err, quartz.ErrJobAlreadyExists):
		return vivid.ErrorJobAlreadyExists.WithMessage(err.Error())
	case errors.Is(err, quartz.ErrJobIsSuspended):
		return vivid.ErrorJobIsSuspended.WithMessage(err.Error())
	case errors.Is(err, quartz.ErrJobIsActive):
		return vivid.ErrorJobIsActive.WithMessage(err.Error())
	default:
		return err
	}
}

func (s *Scheduler) tell(receiver vivid.ActorRef, message vivid.Message, options *vivid.ScheduleOptions) {
	schedulerMessage := &SchedulerMessage{
		Reference: options.Reference,
		Message:   message,
	}
	if receiver.Equals(s.ctx.Ref()) {
		s.ctx.TellSelf(schedulerMessage)
		return
	}
	s.ctx.Tell(receiver, schedulerMessage)
}

func (s *Scheduler) Exists(reference string) bool {
	_, ok := s.jobKeys[reference]
	return ok
}

func (s *Scheduler) Cron(receiver vivid.ActorRef, cron string, message vivid.Message, options ...vivid.ScheduleOption) error {
	opts := vivid.NewScheduleOptions(options...)
	cronTrigger, err := quartz.NewCronTriggerWithLoc(cron, opts.Location)
	if err != nil {
		return schedulerErrorConvert(err)
	}

	jobKey := uniqueJobKey(s.ctx, opts.Reference)
	s.jobKeys[opts.Reference] = jobKey
	job := job.NewFunctionJob(func(ctx context.Context) (any, error) {
		s.tell(receiver, message, opts)
		return nil, nil
	})

	if err := s.scheduler.Schedule(quartz.NewJobDetail(job, jobKey), cronTrigger); err != nil {
		delete(s.jobKeys, opts.Reference)
		return schedulerErrorConvert(err)
	}
	s.ctx.Logger().Debug("scheduler cron scheduled", log.String("ref", s.ctx.Ref().GetPath()), log.String("receiver", receiver.GetPath()), log.String("cron", cron), log.String("messageType", fmt.Sprintf("%T", message)))
	return nil
}

func (s *Scheduler) Once(receiver vivid.ActorRef, delay time.Duration, message vivid.Message, options ...vivid.ScheduleOption) error {
	opts := vivid.NewScheduleOptions(options...)
	onceTrigger := quartz.NewRunOnceTrigger(delay)
	jobKey := uniqueJobKey(s.ctx, opts.Reference)
	s.jobKeys[opts.Reference] = jobKey
	job := job.NewFunctionJob(func(ctx context.Context) (any, error) {
		s.tell(receiver, message, opts)
		return nil, nil
	})

	if err := s.scheduler.Schedule(quartz.NewJobDetail(job, jobKey), onceTrigger); err != nil {
		delete(s.jobKeys, opts.Reference)
		return schedulerErrorConvert(err)
	}
	s.ctx.Logger().Debug("scheduler once scheduled", log.String("ref", s.ctx.Ref().GetPath()), log.String("receiver", receiver.GetPath()), log.Duration("delay", delay), log.String("messageType", fmt.Sprintf("%T", message)))
	return nil
}

func (s *Scheduler) Loop(receiver vivid.ActorRef, interval time.Duration, message vivid.Message, options ...vivid.ScheduleOption) error {
	opts := vivid.NewScheduleOptions(options...)
	loopTrigger := quartz.NewSimpleTrigger(interval)
	jobKey := uniqueJobKey(s.ctx, opts.Reference)
	s.jobKeys[opts.Reference] = jobKey
	job := job.NewFunctionJob(func(ctx context.Context) (any, error) {
		s.tell(receiver, message, opts)
		return nil, nil
	})
	if err := s.scheduler.Schedule(quartz.NewJobDetail(job, jobKey), loopTrigger); err != nil {
		delete(s.jobKeys, opts.Reference)
		return schedulerErrorConvert(err)
	}
	s.ctx.Logger().Debug("scheduler loop scheduled", log.String("ref", s.ctx.Ref().GetPath()), log.String("receiver", receiver.GetPath()), log.Duration("interval", interval), log.String("messageType", fmt.Sprintf("%T", message)))
	return nil
}

func (s *Scheduler) Cancel(reference string) error {
	jobKey, ok := s.jobKeys[reference]
	if !ok {
		return vivid.ErrorNotFound.WithMessage(reference)
	}
	delete(s.jobKeys, reference)
	if err := s.scheduler.DeleteJob(jobKey); err != nil {
		return schedulerErrorConvert(err)
	}
	s.ctx.Logger().Debug("scheduler cancel scheduled", log.String("ref", s.ctx.Ref().GetPath()), log.String("reference", reference))
	return nil
}
