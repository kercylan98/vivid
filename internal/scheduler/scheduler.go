package scheduler

import (
	"context"

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
