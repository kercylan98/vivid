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

	// ErrFutureTimeout 表示 Future 等待超时异常。
	// 常见于调用 Future.Result()/Wait() 时，在指定的超时时间内未等到目标应答消息，导致操作超时。
	// 业务代码可通过判定该错误，实现超时兜底、重试机制等。
	ErrorFutureTimeout = RegisterError(110000, "future timeout")

	// ErrorFutureMessageTypeMismatch 表示 Future 收到不符合预期类型的消息异常。
	// 当通过泛型声明的 Future[期望类型]，但实际收到的消息类型与声明不符时抛出该异常。
	// 业务方可通过判定该错误实现类型安全保护与异常处理。
	ErrorFutureMessageTypeMismatch = RegisterError(110001, "future message type mismatch")
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
	return &Error{
		code: e.code,
		msg:  fmt.Sprintf("%s: %s", e.msg, err.Error()),
		err:  fmt.Errorf("%s: %w", e.msg, err),
	}
}

func (e *Error) Unwrap() error {
	if e.err == nil {
		return e
	}
	return e.err
}

func (e *Error) Is(target error) bool {
	if e.err == nil {
		var err *Error
		if errors.As(target, &err) {
			return e.code == err.code
		}
		return errors.Is(e, target)
	}
	return errors.Is(e.err, target)
}

func (e *Error) As(target any) bool {
	if e.err == nil {
		return errors.As(e, target)
	}
	return errors.As(e.err, target)
}
