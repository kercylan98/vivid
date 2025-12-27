package messages

import (
	"encoding/gob"
	"fmt"
	"reflect"
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
	// RegisterInternalMessage 的泛型参数应为“指针类型”，框架内部会自动取 Elem().Elem() 得到实际消息结构体类型
	RegisterInternalMessage[*NoneArgsCommandMessage]("NoneArgsCommandMessage", onNoneArgsCommandMessageReader, onNoneArgsCommandMessageWriter)
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
