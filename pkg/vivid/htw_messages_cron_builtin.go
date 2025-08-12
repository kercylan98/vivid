//go:build !cron_robfig
// +build !cron_robfig

package vivid

import (
	"strconv"
	"strings"
	"time"
)

// 简易 interval 调度：每 d 触发一次
type intervalSchedule struct{ d time.Duration }

func (s intervalSchedule) Next(t time.Time) time.Time {
	if s.d <= 0 {
		return time.Time{}
	}
	// 舍去到整倍数边界，再加一个周期
	base := t.Truncate(s.d)
	if base.Before(t) || base.Equal(t) {
		base = base.Add(s.d)
	}
	return base
}

// 解析极简 Cron：支持
// - 6 字段，第一字段（秒）为 "*/N" 或 "X/N"（如 "1/1"、"*/5"）视为每 N 秒
// - @every Ns/Nm/Nh
// 其他格式返回零计划（不触发）
type builtinCron struct{ spec string }

func (b builtinCron) Next(time.Time) time.Time { return time.Time{} }

func parseEvery(spec string) (time.Duration, bool) {
	if !strings.HasPrefix(spec, "@every ") {
		return 0, false
	}
	d, err := time.ParseDuration(strings.TrimPrefix(spec, "@every "))
	if err != nil || d <= 0 {
		return 0, false
	}
	return d, true
}

func parseCronSpecRobfig(spec string) (cronSchedule, error) {
	if d, ok := parseEvery(spec); ok {
		return intervalSchedule{d: d}, nil
	}
	fields := strings.Fields(spec)
	if len(fields) >= 6 {
		sec := fields[0]
		// 支持 "*/N" 或 "X/N"
		if strings.Contains(sec, "/") {
			parts := strings.SplitN(sec, "/", 2)
			if len(parts) == 2 {
				n, err := strconv.Atoi(parts[1])
				if err == nil && n > 0 {
					return intervalSchedule{d: time.Duration(n) * time.Second}, nil
				}
			}
		}
	}
	// 默认不触发
	return builtinCron{spec: spec}, nil
}
