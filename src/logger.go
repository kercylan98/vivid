package vivid

import "log/slog"

// Logger 是 slog.Logger 的别名
type Logger = slog.Logger

// defaultLoggerFetcher 是默认的 LoggerFetcher
func defaultLoggerFetcher() *Logger {
	return slog.Default()
}

// LoggerFetcher 是 slog.Logger 的提取器，它允许在运行时调整日志记录器
type LoggerFetcher interface {
	// Fetch 获取日志记录器
	Fetch() *Logger
}

// LoggerFetcherFn 是一个函数类型的 LoggerFetcher
type LoggerFetcherFn func() *Logger

// Fetch 获取日志记录器
func (f LoggerFetcherFn) Fetch() *Logger {
	return f()
}
