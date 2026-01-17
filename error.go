package vivid

import (
	"errors"
	"fmt"
	"sync"

	"github.com/kercylan98/vivid/internal/messages"
)

var (
	// ErrorException 表示异常错误。
	// 常见于系统内部错误、未预料到的异常情况等。
	// 业务代码可通过判定该错误，实现兜底处理、日志记录等。
	ErrorException = RegisterError(-1, "exception") // 异常错误

	// ErrorNotFound 表示未找到错误。
	// 常见于查询不存在的资源时。
	// 业务代码可通过判定该错误，实现兜底处理、日志记录等。
	ErrorNotFound = RegisterError(100000, "not found")
)

var (
	// ErrFutureTimeout 表示 Future 等待超时异常。
	// 常见于调用 Future.Result()/Wait() 时，在指定的超时时间内未等到目标应答消息，导致操作超时。
	// 业务代码可通过判定该错误，实现超时兜底、重试机制等。
	ErrorFutureTimeout = RegisterError(110000, "future timeout")

	// ErrorFutureMessageTypeMismatch 表示 Future 收到不符合预期类型的消息异常。
	// 当通过泛型声明的 Future[期望类型]，但实际收到的消息类型与声明不符时抛出该异常。
	// 业务方可通过判定该错误实现类型安全保护与异常处理。
	ErrorFutureMessageTypeMismatch = RegisterError(110001, "future message type mismatch")

	// ErrorFutureUnexpectedError 表示 Future 收到意外错误异常。
	// 当 Future 收到意外错误时抛出该异常。
	ErrorFutureUnexpectedError = RegisterError(110002, "future unexpected error")

	// ErrorFutureInvalid 表示 Future 无效的错误。
	// 当 Future 被创建时，传入的 timeout 参数不合法时抛出该异常。
	ErrorFutureInvalid = RegisterError(110003, "future invalid")
)

var (
	// ErrorJobNotFound 表示调度器中未找到指定 Job 的错误。
	// 常用于取消、暂停或恢复任务时目标任务不存在的场景。
	ErrorJobNotFound = RegisterError(120000, "job not found")

	// ErrorIllegalArgument 表示传递给某方法或构造器的参数不合法。
	// 例如配置无效参数、必需参数缺失等场景。
	ErrorIllegalArgument = RegisterError(120001, "illegal argument")

	// ErrorCronParse 表示解析 Cron 表达式失败的错误。
	// 常用于调度任务配置不正确的 cron 表达式时。
	ErrorCronParse = RegisterError(120002, "parse cron expression")

	// ErrorTriggerExpired 表示触发器已过期或不可用的错误。
	// 通常用于任务调度触发超出有效期等场景。
	ErrorTriggerExpired = RegisterError(120003, "trigger has expired")

	// ErrorIllegalState 表示操作违背当前状态机或上下文逻辑的错误。
	// 例如状态变更非法、资源不可用等场景。
	ErrorIllegalState = RegisterError(120004, "illegal state")

	// ErrorQueueEmpty 表示队列为空导致无法继续操作的异常。
	// 常用于队列消费、任务弹出、资源获取等为空时。
	ErrorQueueEmpty = RegisterError(120005, "queue is empty")

	// ErrorJobAlreadyExists 表示尝试注册或调度一个已存在的任务时的异常。
	ErrorJobAlreadyExists = RegisterError(120006, "job already exists")

	// ErrorJobIsSuspended 表示对已处于挂起状态的 Job 执行挂起等操作时的异常。
	ErrorJobIsSuspended = RegisterError(120007, "job is suspended")

	// ErrorJobIsActive 表示对已处于活动状态的 Job 执行激活等操作时的异常。
	ErrorJobIsActive = RegisterError(120008, "job is active")

	// ErrorActorRefAddressMismatch 表示 ActorRef 地址不匹配的错误。
	ErrorActorRefAddressMismatch = RegisterError(120009, "actor ref address mismatch")
)

var (
	// ErrorRefEmpty 表示 ActorRef 为空的错误。
	ErrorRefEmpty = RegisterError(130001, "actor ref is empty")

	// ErrorRefFormat 表示 ActorRef 格式非法（例如缺少地址或路径）的错误。
	ErrorRefFormat = RegisterError(130002, "actor ref must contain address and path")

	// ErrorRefInvalidAddress 表示 ActorRef 地址部分不合法的错误。
	ErrorRefInvalidAddress = RegisterError(130003, "actor ref address is invalid")

	// ErrorRefInvalidPath 表示 ActorRef 路径部分不合法的错误。
	ErrorRefInvalidPath = RegisterError(130004, "actor ref path is invalid")

	// ErrorRefNilAgent 表示 AgentRef 为空（无效引用）的错误。
	ErrorRefNilAgent = RegisterError(130005, "agent ref is nil")
)

var _ error = (*Error)(nil)
var codeOfError = make(map[int32]*Error)
var codeOfErrorMu sync.RWMutex

func init() {
	messages.RegisterInternalMessage[*Error]("Error", errorReader, errorWriter)
}

func RegisterError(code int32, msg string) *Error {
	codeOfErrorMu.Lock()
	defer codeOfErrorMu.Unlock()

	if e, ok := codeOfError[code]; ok {
		panic(fmt.Sprintf("error code %d already registered: %s", code, e.Error()))
	}

	e := &Error{
		code: code,
		msg:  msg,
	}
	codeOfError[code] = e
	return e
}

func QueryError(code int32) *Error {
	codeOfErrorMu.RLock()
	defer codeOfErrorMu.RUnlock()

	if e, ok := codeOfError[code]; ok {
		return e
	}
	return nil
}

// Error 是可以在分布式环境中传播的错误类型
type Error struct {
	code int32  // 错误码
	msg  string // 错误消息
	err  error  // 底层错误（用于错误链，不序列化）
}

func errorReader(message any, reader *messages.Reader, codec messages.Codec) error {
	m := message.(*Error)
	return reader.ReadInto(&m.code, &m.msg)
}

func errorWriter(message any, writer *messages.Writer, codec messages.Codec) error {
	m := message.(*Error)
	return writer.WriteFrom(m.code, m.msg)
}

func (e *Error) Error() string {
	return fmt.Sprintf("[vivid: %d] %s", e.code, e.msg)
}

func (e *Error) GetCode() int32 {
	return e.code
}

func (e *Error) GetMessage() string {
	return e.msg
}

func (e *Error) With(err error) *Error {
	if err == nil {
		return e
	}
	return &Error{
		code: e.code,
		msg:  fmt.Sprintf("%s: %s", e.msg, err.Error()),
		err:  err,
	}
}

func (e *Error) WithMessage(msg string) *Error {
	if msg == "" {
		return e
	}
	return &Error{
		code: e.code,
		msg:  fmt.Sprintf("%s: %s", e.msg, msg),
		err:  e,
	}
}

func (e *Error) Unwrap() error {
	return e.err
}

func (e *Error) Is(target error) bool {
	if target == nil {
		return false
	}
	if err, ok := target.(*Error); ok {
		return e.code == err.code
	}
	if e.err != nil {
		return errors.Is(e.err, target)
	}
	return false
}

func (e *Error) As(target any) bool {
	if target == nil {
		return false
	}
	if errTarget, ok := target.(**Error); ok {
		*errTarget = e
		return true
	}
	if e.err != nil {
		return errors.As(e.err, target)
	}
	return false
}
