package vivid

import "github.com/kercylan98/go-log/log"

var defaultLoggerProvider = log.ProviderFn(func() log.Logger {
	return log.GetDefault()
})
