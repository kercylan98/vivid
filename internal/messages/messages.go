package messages

import (
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
		return fmt.Errorf("outside message desc writer is not implemented")
	},
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
	tof := reflect.TypeOf((*T)(nil)).Elem()
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
	tof := reflect.TypeOf(message)
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
	return desc.writer(message, writer)
}

func DeserializeRemotingMessage(reader *Reader, desc *MessageDesc) (any, error) {
	message := desc.Instance()
	err := desc.reader(message, reader)
	if err != nil {
		return nil, err
	}
	return message, nil
}
