package console

import (
	"time"
)

func NewOptions(options ...Option) *Options {
	opts := &Options{
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	for _, option := range options {
		option(opts)
	}
	return opts
}

// Option 控制台 HTTP 服务配置选项。
type Option = func(options *Options)

// Options 控制台 HTTP 服务配置，用于 NewServer。
type Options struct {
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func WithReadTimeout(readTimeout time.Duration) Option {
	return func(options *Options) {
		options.ReadTimeout = readTimeout
	}
}

func WithWriteTimeout(writeTimeout time.Duration) Option {
	return func(options *Options) {
		options.WriteTimeout = writeTimeout
	}
}
