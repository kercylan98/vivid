package vivid

import (
	"errors"
	"fmt"
	"sync"

	"github.com/kercylan98/vivid/internal/messages"
)

// 系统与资源相关错误。可用 errors.Is 判定并做兜底或日志处理。
var (
	ErrorException                 = RegisterError(-1, "exception")                        // 未分类内部异常
	ErrorNotFound                  = RegisterError(100000, "not found")                    // 资源不存在
	ErrorActorSystemAlreadyStarted = RegisterError(100001, "actor system already started") // 重复 Start
	ErrorActorSystemAlreadyStopped = RegisterError(100002, "actor system already stopped") // 重复 Stop
	ErrorActorSystemStartFailed    = RegisterError(100003, "actor system start failed")    // 启动失败
	ErrorActorSystemStopFailed     = RegisterError(100004, "actor system stop failed")     // 停止失败
	ErrorActorSystemNotStarted     = RegisterError(100005, "actor system not started")     // 未启动时调用 Stop
	ErrorActorSystemStopped        = RegisterError(100006, "actor system stopped")         // 系统已停止

	ErrorActorDeaded          = RegisterError(100100, "actor deaded")           // Actor 已死亡
	ErrorActorAlreadyExists   = RegisterError(100101, "actor already exists")   // Actor 已存在
	ErrorActorSpawnFailed     = RegisterError(100102, "actor spawn failed")     // Actor 创建失败
	ErrorActorPrelaunchFailed = RegisterError(100103, "actor prelaunch failed") // Actor 预启动失败
)

// Future 与消息相关错误。
var (
	ErrorFutureTimeout             = RegisterError(110000, "future timeout")               // Result/Wait 超时未收到应答
	ErrorFutureMessageTypeMismatch = RegisterError(110001, "future message type mismatch") // 应答类型与泛型声明不一致
	ErrorFutureUnexpectedError     = RegisterError(110002, "future unexpected error")      // 收到非预期错误
	ErrorFutureInvalid             = RegisterError(110003, "future invalid")               // 创建时 timeout 非法
	ErrorInvalidMessageLength      = RegisterError(110004, "invalid message length")       // 消息长度非法
	ErrorReadMessageBufferFailed   = RegisterError(110005, "read message buffer failed")   // 读消息缓冲失败
)

// 参数、调度与状态相关错误。
var (
	ErrorIllegalArgument = RegisterError(120000, "illegal argument")      // 参数无效或缺失
	ErrorCronParse       = RegisterError(120001, "parse cron expression") // Cron 表达式解析失败
	//ErrorTriggerExpired          = RegisterError(120002, "trigger has expired")        // 触发器已过期
	//ErrorIllegalState            = RegisterError(120003, "illegal state")              // 违反当前状态或上下文
	//ErrorQueueEmpty              = RegisterError(120004, "queue is empty")             // 队列空无法出队
	//ErrorJobAlreadyExists        = RegisterError(120005, "job already exists")         // 任务已注册
	//ErrorJobIsSuspended          = RegisterError(120006, "job is suspended")           // Job 已挂起时再次挂起
	//ErrorJobIsActive             = RegisterError(120007, "job is active")              // Job 已激活时再次激活
	//ErrorActorRefAddressMismatch = RegisterError(120008, "actor ref address mismatch") // ActorRef 地址不一致
)

// ActorRef 相关错误。
var (
	ErrorRefEmpty          = RegisterError(130001, "actor ref is empty")                      // ActorRef 为空
	ErrorRefFormat         = RegisterError(130002, "actor ref must contain address and path") // 缺少 address 或 path
	ErrorRefInvalidAddress = RegisterError(130003, "actor ref address is invalid")            // 地址非法
	ErrorRefInvalidPath    = RegisterError(130004, "actor ref path is invalid")               // 路径非法
	ErrorRefNilAgent       = RegisterError(130005, "agent ref is nil")                        // AgentRef 为 nil
)

// Remoting 相关错误。
var (
	ErrorRemotingMessageSendFailed   = RegisterError(140000, "remoting message send failed")   // 消息发送失败
	ErrorRemotingMessageEncodeFailed = RegisterError(140001, "remoting message encode failed") // 消息编码失败
	ErrorRemotingMessageDecodeFailed = RegisterError(140002, "remoting message decode failed") // 消息解码失败
	ErrorRemotingMessageHandleFailed = RegisterError(140003, "remoting message handle failed") // 消息处理失败
)

// Cluster 相关错误。
var (
	ErrorClusterNameMismatch = RegisterError(150000, "cluster name mismatch") // 集群名称不匹配
	ErrorClusterDisabled     = RegisterError(150001, "cluster disabled")      // 集群已禁用
)

var _ error = (*Error)(nil)
var codeOfError = make(map[int32]*Error)
var codeOfErrorMu sync.RWMutex

func init() {
	messages.RegisterInternalMessage[*Error]("Error", errorReader, errorWriter)
}

// RegisterError 在全局注册一个错误类型并返回其实例。
//
// code 为唯一错误码，用于跨进程/节点识别；msg 为人类可读描述，会出现在 Error() 中。
// 若 code 已被注册则 panic。返回的 *Error 可安全复用于 errors.Is/As 判定及序列化传播。
// 通常仅在包 init 或启动阶段调用。
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

// QueryError 根据错误码从全局注册表查找并返回对应的 *Error。
//
// 若 code 未注册则返回 nil。用于反序列化或跨节点错误码还原为 *Error 实例。
func QueryError(code int32) *Error {
	codeOfErrorMu.RLock()
	defer codeOfErrorMu.RUnlock()

	if e, ok := codeOfError[code]; ok {
		return e
	}
	return nil
}

// Error 可在分布式环境中序列化传播，支持 errors.Is/As 与错误链。
type Error struct {
	code int32  // 错误码
	msg  string // 可读描述
	err  error  // 包装的底层错误（不参与序列化）
}

// errorReader 从 reader 反序列化 Error 的 code 与 msg 到 message，供内部消息框架使用。
func errorReader(message any, reader *messages.Reader, codec messages.Codec) error {
	m := message.(*Error)
	return reader.ReadInto(&m.code, &m.msg)
}

// errorWriter 将 Error 的 code 与 msg 序列化写入 writer，供内部消息框架使用。
func errorWriter(message any, writer *messages.Writer, codec messages.Codec) error {
	m := message.(*Error)
	return writer.WriteFrom(m.code, m.msg)
}

// Error 实现 error 接口，返回格式为 "[vivid: <code>] <msg>" 的字符串。
func (e *Error) Error() string {
	return fmt.Sprintf("[vivid: %d] %s", e.code, e.msg)
}

// GetCode 返回错误的数字码，用于日志、监控或跨节点一致判定。
func (e *Error) GetCode() int32 {
	return e.code
}

// GetMessage 返回错误的可读描述（不含前缀与错误码）。
func (e *Error) GetMessage() string {
	return e.msg
}

// With 包装底层错误 err，生成新的 *Error：msg 变为 "原 msg: err.Error()"，Unwrap 返回 err。
// 若 err 为 nil 则返回接收者本身。用于构建错误链并保留 errors.Is/As 的递归判定。
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

// WithMessage 在原有描述后追加一段说明 msg，生成新的 *Error，Unwrap 返回接收者。
// 若 msg 为空则返回接收者本身。不改变错误码，仅丰富可读信息。
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

// Unwrap 返回通过 With 包装的底层错误，供 errors.Unwrap 及 errors.Is/As 使用；未包装时返回 nil。
func (e *Error) Unwrap() error {
	return e.err
}

// Is 实现 errors 包中的错误匹配：若 target 为 *Error 则比较错误码；否则对包装链递归调用 errors.Is。
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

// As 实现 errors 包中的类型断言：若 target 为 **Error 则赋值为当前 *Error；否则对包装链递归调用 errors.As。
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
