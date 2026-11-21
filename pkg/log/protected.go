package log

// Protect 返回隔离 panic 的日志器，不修改传入的实例。
// 输出失败时丢弃本条日志；With 或 WithGroup panic 或返回 nil 时保留原上下文。
// 日志仍同步执行，无法中断阻塞或终止进程的日志实现。
func Protect(logger Logger) Logger {
	switch logger := logger.(type) {
	case *protectedLogger:
		return logger
	case *VividLogger:
		if logger != nil {
			if logger.protected {
				return logger
			}
			clone := *logger
			clone.protected = true
			return &clone
		}
	case *SLogLogger:
		if logger != nil {
			if logger.protected {
				return logger
			}
			clone := *logger
			clone.protected = true
			return &clone
		}
	}
	return &protectedLogger{logger: logger}
}

type protectedLogger struct {
	logger Logger
}

// recoverLogPanic 不再使用发生异常的日志器报告自身错误，避免递归失败。
func recoverLogPanic() {
	_ = recover()
}

func (l *protectedLogger) Debug(message string, args ...any) {
	defer recoverLogPanic()
	l.logger.Debug(message, args...)
}

func (l *protectedLogger) Info(message string, args ...any) {
	defer recoverLogPanic()
	l.logger.Info(message, args...)
}

func (l *protectedLogger) Warn(message string, args ...any) {
	defer recoverLogPanic()
	l.logger.Warn(message, args...)
}

func (l *protectedLogger) Error(message string, args ...any) {
	defer recoverLogPanic()
	l.logger.Error(message, args...)
}

func (l *protectedLogger) With(args ...any) (result Logger) {
	result = l
	defer recoverLogPanic()
	if child := l.logger.With(args...); child != nil {
		result = Protect(child)
	}
	return
}

func (l *protectedLogger) WithGroup(group string) (result Logger) {
	result = l
	defer recoverLogPanic()
	if child := l.logger.WithGroup(group); child != nil {
		result = Protect(child)
	}
	return
}
