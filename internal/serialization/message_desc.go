package serialization

import (
	"fmt"
	"reflect"
)

func newMessageDesc(class string, name string, typeOf reflect.Type, encoder MessageEncoder, decoder MessageDecoder) *MessageDesc {
	return &MessageDesc{
		fullname: generateMessageDescFullname(class, name),
		class:    class,
		name:     name,
		typeOf:   typeOf,
		encoder:  encoder,
		decoder:  decoder,
	}
}

type MessageDesc struct {
	fullname string       // 消息的唯一标识符
	class    string       // 消息所属的类别
	name     string       // 消息的唯一名称
	typeOf   reflect.Type // 消息的类型
	encoder  MessageEncoder
	decoder  MessageDecoder
}

func generateMessageDescFullname(class string, name string) string {
	return fmt.Sprintf("%s.%s", class, name)
}

func (d *MessageDesc) Class() string {
	return d.class
}

func (d *MessageDesc) Name() string {
	return d.name
}

func (d *MessageDesc) Instantiate() any {
	if d.typeOf.Kind() == reflect.Ptr {
		return reflect.New(d.typeOf.Elem()).Interface()
	}
	return reflect.New(d.typeOf).Elem().Interface()
}

func (d *MessageDesc) FullName() string {
	return d.fullname
}

func (d *MessageDesc) Encode(writer *Writer, message any) error {
	return d.encoder.Encode(writer, message)
}

func (d *MessageDesc) Decode(reader *Reader, message any) error {
	return d.decoder.Decode(reader, message)
}
