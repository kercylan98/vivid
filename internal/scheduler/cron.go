package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/robfig/cron/v3"
)

var cronParser = cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

var cronDescriptors = map[string]string{
	"@yearly":   "0 0 0 1 1 *",
	"@annually": "0 0 0 1 1 *",
	"@monthly":  "0 0 0 1 * *",
	"@weekly":   "0 0 0 * * 0",
	"@daily":    "0 0 0 * * *",
	"@midnight": "0 0 0 * * *",
	"@hourly":   "0 0 * * * *",
}

type cronYearRange struct {
	from, to, step int
}

type cronSchedule struct {
	plan  *cron.SpecSchedule
	years []cronYearRange // 空表示不限制年份，不按年份数量分配存储
}

// ParseCron 解析含秒的六段表达式及可选年份，日历计算不经过纳秒时间戳。
func ParseCron(expression string, location *time.Location) (cron.Schedule, error) {
	if location == nil {
		location = time.Local
	}
	expression = strings.TrimSpace(expression)
	if expanded, ok := cronDescriptors[expression]; ok {
		expression = expanded
	}
	fields := strings.Fields(expression)
	if len(fields) != 6 && len(fields) != 7 {
		return nil, vivid.ErrorCronParse.WithMessage("expected six fields and an optional year")
	}
	for index, field := range fields {
		if index != 3 && index != 5 && strings.ContainsRune(field, '?') {
			return nil, vivid.ErrorCronParse.WithMessage("? is only allowed for day of month or day of week")
		}
	}
	var years []cronYearRange
	if len(fields) == 7 && fields[6] != "*" {
		var err error
		years, err = parseCronYears(fields[6])
		if err != nil {
			return nil, vivid.ErrorCronParse.With(err)
		}
	}
	// 日与星期只限制其中一个，避免两者同时指定时产生难以察觉的 OR 语义。
	if fields[3] != "*" && fields[3] != "?" && fields[5] != "*" && fields[5] != "?" {
		return nil, vivid.ErrorCronParse.WithMessage("day of month and day of week cannot both be restricted")
	}
	for _, index := range []int{3, 5} {
		if fields[index] == "?" {
			fields[index] = "*"
		}
	}
	plan, err := cronParser.Parse(strings.Join(fields[:6], " "))
	if err != nil {
		return nil, vivid.ErrorCronParse.With(err)
	}
	spec := plan.(*cron.SpecSchedule)
	spec.Location = location
	return &cronSchedule{plan: spec, years: years}, nil
}

func parseCronYears(field string) ([]cronYearRange, error) {
	parts := strings.Split(field, ",")
	years := make([]cronYearRange, 0, len(parts))
	for _, part := range parts {
		base, stepText, stepped := strings.Cut(part, "/")
		item := cronYearRange{from: 1970, to: 9999, step: 1}
		var err error
		if base != "*" {
			from, to, ranged := strings.Cut(base, "-")
			item.from, err = strconv.Atoi(from)
			if err == nil && ranged {
				item.to, err = strconv.Atoi(to)
			} else if !stepped {
				item.to = item.from
			}
		}
		if err == nil && stepped {
			item.step, err = strconv.Atoi(stepText)
		}
		if err != nil || item.from < 1970 || item.to > 9999 || item.from > item.to || item.step < 1 || item.step > 9999 {
			return nil, fmt.Errorf("invalid cron year: %s", part)
		}
		years = append(years, item)
	}
	return years, nil
}

func (s *cronSchedule) nextYear(year int) int {
	if year > 9999 {
		return 0
	}
	if len(s.years) == 0 {
		return year
	}
	next := 0
	for _, item := range s.years {
		candidate := max(year, item.from)
		offset := (candidate - item.from) % item.step
		if offset != 0 {
			candidate += item.step - offset
		}
		if candidate <= item.to && (next == 0 || candidate < next) {
			next = candidate
		}
	}
	return next
}

func (s *cronSchedule) Next(after time.Time) time.Time {
	after = after.In(s.plan.Location)
	year := max(1970, after.Year())
	for {
		year = s.nextYear(year)
		if year == 0 {
			return time.Time{}
		}
		start := after
		if year > after.Year() {
			start = time.Date(year, 1, 1, 0, 0, 0, 0, s.plan.Location).Add(-time.Nanosecond)
		}
		next := s.plan.Next(start)
		if next.IsZero() {
			// 解析库只向前搜索五年，跨越非闰世纪的 2 月 29 日可能相隔八年。
			next = s.plan.Next(time.Date(start.Year()+5, 1, 1, 0, 0, 0, 0, s.plan.Location).Add(-time.Nanosecond))
		}
		if next.IsZero() || next.Year() > 9999 {
			return time.Time{}
		}
		if s.nextYear(next.Year()) == next.Year() {
			return next
		}
		year = next.Year() + 1
	}
}
