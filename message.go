package vivid

import (
	"fmt"

	"github.com/kercylan98/vivid/internal/messages"
)

func init() {
	messages.RegisterInternalMessage[*OnLaunch]("OnLaunch", onLaunchReader, onLaunchWriter)
	messages.RegisterInternalMessage[*OnKill]("OnKill", onKillReader, onKillWriter)
	messages.RegisterInternalMessage[*OnKilled]("OnKilled", onKilledReader, onKilledWriter)
	messages.RegisterInternalMessage[*PipeResult]("PipeResult", pipeResultReader, pipeResultWriter)
}

// Message 表示可被 Actor 系统传递和处理的消息类型。
// 推荐用户自定义消息采用结构体类型，以增强类型安全与后续扩展能力。
// 系统内部消息如 OnLaunch、OnKill、OnKilled 也采用此规范。
// 使用 any 类型以便适应不同场景的消息传递和泛型组合。
type Message = any

// OnLaunch 表示 Actor 生命周期初始化时自动下发的启动消息。
// 每个 Actor 启动时会收到一条 OnLaunch，可在对应处理逻辑中完成初始化操作。
// 此结构体无字段，仅用于启动事件的识别。
type OnLaunch struct{}

func onLaunchReader(message any, reader *messages.Reader, codec messages.Codec) error {
	return nil
}

func onLaunchWriter(message any, writer *messages.Writer, codec messages.Codec) error {
	return nil
}

// OnKill 表示对 Actor 发起的终止请求。
// 该消息用于有序、安全地关闭 Actor，支持 poison-pill 模式和常规 kill 流程。
type OnKill struct {
	Killer ActorRef // 发起终止请求的 ActorRef
	Reason string   // 终止原因描述，便于追踪和日志分析。
	Poison bool     // 是否采用毒杀模式，true 时立即销毁，不处理剩余队列，false 时常规优雅下线。
}

func onKillReader(message any, reader *messages.Reader, codec messages.Codec) error {
	m := message.(*OnKill)
	return reader.ReadInto(&m.Killer, &m.Reason, &m.Poison)
}

func onKillWriter(message any, writer *messages.Writer, codec messages.Codec) error {
	m := message.(*OnKill)
	return writer.WriteFrom(m.Killer, m.Reason, m.Poison)
}

// OnKilled 表示 Actor 已被终止后的系统事件通知。
// 当 Actor 资源释放完毕，相关方可根据该信号进行收尾处理。
// 无需携带字段，仅作状态事件标志。
type OnKilled struct {
	Ref ActorRef // 被终止的 ActorRef
}

func onKilledReader(message any, reader *messages.Reader, codec messages.Codec) error {
	m := message.(*OnKilled)
	return reader.ReadInto(&m.Ref)
}

func onKilledWriter(message any, writer *messages.Writer, codec messages.Codec) error {
	m := message.(*OnKilled)
	return writer.WriteFrom(m.Ref)
}

type StreamEvent any

type PipeResult struct {
	Id      string  // 管道ID
	Message Message // 管道结果消息
	Error   error   // 管道结果错误
}

func pipeResultReader(message any, reader *messages.Reader, codec messages.Codec) (err error) {
	m := message.(*PipeResult)

	var errorCode int32
	var errorMessage string

	if m.Message, err = reader.ReadMessage(codec); err != nil {
		return err
	}

	if err = reader.ReadInto(&m.Id, &errorCode, &errorMessage); err != nil {
		return err
	}

	if errorCode != 0 {
		var foundError = QueryError(errorCode)
		if foundError == nil {
			foundError = ErrorException.With(fmt.Errorf("error code %d not found, message: %s", errorCode, errorMessage))
		}
		m.Error = foundError
	}
	return nil
}

func pipeResultWriter(message any, writer *messages.Writer, codec messages.Codec) (err error) {
	m := message.(*PipeResult)

	if err = writer.WriteMessage(m.Message, codec); err != nil {
		return err
	}

	var errorCode int32
	var errorMessage string
	switch err := m.Error.(type) {
	case nil:
	case *Error:
		errorCode = err.GetCode()
		errorMessage = err.GetMessage()
	default:
		errorCode = ErrorException.GetCode()
		errorMessage = ErrorException.With(err).GetMessage()
	}

	return writer.WriteFrom(
		m.Id,                    // PipeID
		errorCode, errorMessage, // 错误码和错误消息
	)
}

func (p *PipeResult) GetId() string {
	return p.Id
}

func (p *PipeResult) IsSuccess() bool {
	return p.Error == nil
}

func (p *PipeResult) IsError() bool {
	return p.Error != nil
}

func (p *PipeResult) GetMessage() Message {
	return p.Message
}

func (p *PipeResult) GetError() error {
	return p.Error
}
