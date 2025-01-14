package options

// LogicOptions 定义了支持简单逻辑的选项接口，它被用于条件判断、逻辑判断等。
type LogicOptions[Fetcher, Options any] interface {
	// If 如果 expression 为真，则执行 then 函数
	If(expression func(config Fetcher) bool, then func(options Options)) Options

	// IfElse 如果 expression 为真，则执行 then 函数，否则执行 elseFunc 函数
	IfElse(expression func(config Fetcher) bool, then func(options Options), elseFunc func(options Options)) Options
}

func NewLogicOptions[Fetcher, Options any](fetcher Fetcher, options Options) LogicOptions[Fetcher, Options] {
	return &logicOptions[Fetcher, Options]{
		fetcher: fetcher,
		options: options,
	}
}

type logicOptions[Fetcher, Options any] struct {
	fetcher Fetcher
	options Options
}

func (d *logicOptions[Fetcher, Options]) If(expression func(config Fetcher) bool, then func(options Options)) Options {
	if expression(d.fetcher) {
		then(d.options)
	}
	return d.options
}

func (d *logicOptions[Fetcher, Options]) IfElse(expression func(config Fetcher) bool, then func(options Options), elseFunc func(options Options)) Options {
	if expression(d.fetcher) {
		then(d.options)
	} else {
		elseFunc(d.options)
	}
	return d.options
}
