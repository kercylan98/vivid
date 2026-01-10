package scheduler

import "github.com/reugn/go-quartz/logger"

var (
	_ logger.Logger = (*discordLogger)(nil)
)

type discordLogger struct{}

func (d *discordLogger) Debug(msg string, args ...any) {}

func (d *discordLogger) Error(msg string, args ...any) {}

func (d *discordLogger) Info(msg string, args ...any) {}

func (d *discordLogger) Trace(msg string, args ...any) {}

func (d *discordLogger) Warn(msg string, args ...any) {}
