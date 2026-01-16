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

// CustomMessageReader 定义了自定义消息的读取函数签名，用于自定义消息的解码过程。
//
// 参数：
//   - message: any
//     需要被填充的自定义消息对象（通常为指向结构体的指针），实现时应将数据从 reader 解码填充至该对象。
//   - reader: *messages.Reader
//     字节流读取器，已实现基础类型和结构体成员的二进制读取，可通过 ReadInto 解码结构体字段。
//   - codec: messages.Codec
//     编解码器接口，支持更复杂或自定义的消息格式解析（如 JSON、ProtoBuf），如无特殊需求可忽略。
//
// 返回值：
//   - error
//     读取或解码出错时返回错误对象，否则返回 nil。
type CustomMessageReader = messages.InternalMessageReader

// CustomMessageWriter 定义了自定义消息的写入函数签名，用于自定义消息的编码过程。
//
// 参数：
//   - message: any
//     需要序列化写入的自定义消息对象（通常为结构体指针），实现时应将其字段写入 writer。
//   - writer: *messages.Writer
//     字节流写入器，已实现常用基础类型与结构体成员的二进制写入，可直接通过 WriteFrom 输出结构体字段。
//   - codec: messages.Codec
//     编解码器接口，可在需要特定序列化格式（如 JSON/ProtoBuf）时使用，普通结构体可直接用 writer。
//
// 返回值：
//   - error
//     序列化/写入发生错误时返回错误对象，否则返回 nil。
type CustomMessageWriter = messages.InternalMessageWriter

// RegisterCustomMessage 注册自定义消息类型至 Actor 系统的内部消息描述表，
// 用于支持远程消息编解码、分布式 Actor 间的跨节点消息传递能力。
//
// 推荐为所有需支持远程通信（Remoting）的用户自定义消息类型进行注册，
// 以确保系统能正确序列化、反序列化对应消息并赋予其唯一的消息名称与读写器。
//
// 注意：如果希望无需注册自定义消息，也可以通过 WithCodec 显式设置 Codec，
// 此时 ActorSystem 将统一采用 Codec 进行所有消息的序列化与反序列化，无需依赖消息注册表。
// 只有在未显式设置 Codec 时，才需为每种自定义消息注册读写器，否则消息将被视为外部消息，无法正确编解码。
//
// 典型用法：
//
//	type MyEvent struct { ... }
//	RegisterCustomMessage[*MyEvent]("MyEvent", myEventReader, myEventWriter)
//
// 参数：
//   - messageName string
//     消息在系统中的唯一名称建议与类型名保持一致，内部注册表将用其区分消息类型。
//   - reader CustomMessageReader
//     消息的自定义读取器，在消息反序列化（如网络到本地）时调用；需实现内容读取和结构体字段赋值。
//   - writer CustomMessageWriter
//     消息的自定义写入器，在消息序列化（本地发送到网络）时调用；需实现内容写出。
//
// 泛型：
//   - T any
//     需注册的自定义消息类型，通常为结构体的指针类型（如 *MyEvent），用于类型安全与解包一致性。
//
// 注意事项：
//   - 注册仅需执行一次，通常在 init 或应用初始化阶段统一登记所有自定义消息。
//   - 否则系统无法支持该消息类型的远程传输（会视为外部消息，编解码将失败，除非已用 WithCodec 设置 Codec）。
func RegisterCustomMessage[T any](messageName string, reader CustomMessageReader, writer CustomMessageWriter) {
	messages.RegisterInternalMessage[T](messageName, reader, writer)
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
