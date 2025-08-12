//go:build cron_robfig
// +build cron_robfig

package vivid

import (
	"time"

	cron "github.com/robfig/cron/v3"
)

type cronScheduleRobfig struct{ schedule cron.Schedule }

func (c cronScheduleRobfig) Next(t time.Time) time.Time { return c.schedule.Next(t) }

func parseCronSpecRobfig(spec string) (cronSchedule, error) {
	sch, err := cron.ParseStandard(spec)
	if err != nil {
		return nil, err
	}
	return cronScheduleRobfig{schedule: sch}, nil
}
