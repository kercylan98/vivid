package scheduler

import (
	"context"

	"github.com/kercylan98/vivid"
	"github.com/reugn/go-quartz/quartz"
)

func NewScheduler(ctx context.Context) *Scheduler {
	quartzScheduler, _ := quartz.NewStdScheduler(quartz.WithLogger(new(discordLogger)))
	quartzScheduler.Start(ctx)
	return &Scheduler{
		scheduler: quartzScheduler,
	}
}

type Scheduler struct {
	scheduler quartz.Scheduler
}

func (s *Scheduler) Schedule(jobDetail *quartz.JobDetail, trigger quartz.Trigger) error {
	return s.scheduler.ScheduleJob(jobDetail, trigger)
}

func (s *Scheduler) PauseJob(jobKey *quartz.JobKey) error {
	return s.scheduler.PauseJob(jobKey)
}

func (s *Scheduler) ResumeJob(jobKey *quartz.JobKey) error {
	return s.scheduler.ResumeJob(jobKey)
}

func (s *Scheduler) DeleteJob(jobKey *quartz.JobKey) error {
	return s.scheduler.DeleteJob(jobKey)
}

func (s *Scheduler) Clear(ctx vivid.ActorContext) error {
	return s.scheduler.Clear()
}

func (s *Scheduler) Stop() {
	s.scheduler.Stop()
}
