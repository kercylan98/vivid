package messages

import (
	"encoding/gob"
	"fmt"
	"reflect"
	"time"
)

var outsideMessageDesc = &MessageDesc{
	typeOf:      nil,
	messageName: "",
	reader: func(message any, reader *Reader) error {
		return fmt.Errorf("outside message desc reader is not implemented")
	},
	writer: func(message any, writer *Writer) error {
		messageType := reflect.TypeOf(message).Elem()
		gob.RegisterName(messageType.Name(), message)
		return fmt.Errorf("outside message desc writer is not implemented")
	},
}

func init() {
	RegisterInternalMessage[*NoneArgsCommandMessage]("NoneArgsCommandMessage", onNoneArgsCommandMessageReader, onNoneArgsCommandMessageWriter)
	RegisterInternalMessage[*PingMessage]("PingMessage", onPingMessageReader, onPingMessageWriter)
	RegisterInternalMessage[*PongMessage]("PongMessage", onPongMessageReader, onPongMessageWriter)
	RegisterInternalMessage[*WatchMessage]("WatchMessage", onWatchMessageReader, onWatchMessageWriter)
	RegisterInternalMessage[*UnwatchMessage]("UnwatchMessage", onUnwatchMessageReader, onUnwatchMessageWriter)
}

type MessageDesc struct {
	typeOf      reflect.Type
	messageName string
	reader      InternalMessageReader
	writer      InternalMessageWriter
}

func (desc *MessageDesc) MessageName() string {
	return desc.messageName
}

func (desc *MessageDesc) MessageTypeOf() reflect.Type {
	return desc.typeOf
}

func (desc *MessageDesc) IsOutside() bool {
	return desc.typeOf == nil || desc.messageName == ""
}

func (desc *MessageDesc) Instance() any {
	return reflect.New(desc.typeOf).Interface()
}

var (
	internalMessageTypeOfDesc = make(map[reflect.Type]*MessageDesc)
	internalMessageNameOfDesc = make(map[string]*MessageDesc)
)

type (
	InternalMessageReader = func(message any, reader *Reader) error
	InternalMessageWriter = func(message any, writer *Writer) error
)

func RegisterInternalMessage[T any](messageName string, reader InternalMessageReader, writer InternalMessageWriter) {
	tof := reflect.TypeOf((*T)(nil)).Elem().Elem()
	desc := &MessageDesc{
		typeOf:      tof,
		messageName: messageName,
		reader:      reader,
		writer:      writer,
	}
	internalMessageTypeOfDesc[tof] = desc
	internalMessageNameOfDesc[messageName] = desc
}

func QueryMessageDesc(message any) *MessageDesc {
	tof := reflect.TypeOf(message).Elem()
	desc, ok := internalMessageTypeOfDesc[tof]
	if ok {
		return desc
	}
	return outsideMessageDesc
}

func QueryMessageDescByName(messageName string) *MessageDesc {
	desc, ok := internalMessageNameOfDesc[messageName]
	if ok {
		return desc
	}
	return outsideMessageDesc
}

func SerializeRemotingMessage(writer *Writer, desc *MessageDesc, message any) error {
	dw := NewWriterFromPool()
	defer ReleaseWriterToPool(dw)
	if err := desc.writer(message, dw); err != nil {
		return err
	}
	// 写入包含长度的数据
	writer.WriteBytesWithLength(dw.Bytes(), 4)
	return nil
}

func DeserializeRemotingMessage(reader *Reader, desc *MessageDesc) (any, error) {
	message := desc.Instance()
	err := desc.reader(message, reader)
	if err != nil {
		return nil, err
	}
	return message, nil
}

// Command 表示命令消息的命令
type Command uint8

const (
	CommandPauseMailbox  Command = iota // 暂停邮箱消息处理
	CommandResumeMailbox                // 恢复邮箱消息处理
)

func (c Command) String() string {
	var name string
	switch c {
	case CommandPauseMailbox:
		name = "CommandPauseMailbox"
	case CommandResumeMailbox:
		name = "CommandResumeMailbox"
	default:
		name = "UnknownCommand"
	}
	return fmt.Sprintf("%s(%d)", name, uint8(c))
}

func (c Command) Build() *NoneArgsCommandMessage {
	return &NoneArgsCommandMessage{Command: c}
}

// NoneArgsCommandMessage 表示没有参数的命令消息，行为由枚举决定
type NoneArgsCommandMessage struct {
	Command Command
}

func onNoneArgsCommandMessageReader(message any, reader *Reader) error {
	m := message.(*NoneArgsCommandMessage)
	ui8 := uint8(m.Command)
	if err := reader.ReadInto(&ui8); err != nil {
		return err
	}
	m.Command = Command(ui8)
	return nil
}

func onNoneArgsCommandMessageWriter(message any, writer *Writer) error {
	m := message.(*NoneArgsCommandMessage)
	return writer.WriteFrom(uint8(m.Command))
}

type PingMessage struct {
	Time time.Time // 发出 Ping 消息的时间
}

func onPingMessageReader(message any, reader *Reader) error {
	m := message.(*PingMessage)
	var unixNano int64
	if err := reader.ReadInto(&unixNano); err != nil {
		return err
	}
	m.Time = time.Unix(0, int64(unixNano))
	return nil
}

func onPingMessageWriter(message any, writer *Writer) error {
	m := message.(*PingMessage)
	return writer.WriteFrom(m.Time.UnixNano())
}

type PongMessage struct {
	Ping        *PingMessage // 对应的 Ping 消息
	RespondTime time.Time    // 响应 Pong 消息的时间
}

func onPongMessageReader(message any, reader *Reader) error {
	m := message.(*PongMessage)
	var pingTime int64
	var respondTime int64
	if err := reader.ReadInto(&pingTime, &respondTime); err != nil {
		return err
	}
	m.Ping = &PingMessage{Time: time.Unix(0, pingTime)}
	m.RespondTime = time.Unix(0, respondTime)
	return nil
}

func onPongMessageWriter(message any, writer *Writer) error {
	m := message.(*PongMessage)
	return writer.WriteFrom(m.Ping.Time.UnixNano(), m.RespondTime.UnixNano())
}

type WatchMessage struct{}

func onWatchMessageReader(message any, reader *Reader) error {
	return nil
}

func onWatchMessageWriter(message any, writer *Writer) error {
	return nil
}

type UnwatchMessage struct {
}

func onUnwatchMessageReader(message any, reader *Reader) error {
	return nil
}

func onUnwatchMessageWriter(message any, writer *Writer) error {
	return nil
}
