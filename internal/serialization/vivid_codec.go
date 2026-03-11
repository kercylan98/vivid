package serialization

import (
	"fmt"
	"reflect"
)

var (
	_ Codec = (*VividCodec)(nil)
)

// VividCodec 是 vivid 内部提供的编解码器，它被用于内部消息的序列化与反序列化，同时也支持外部消息注册接管序列化与反序列化
type VividCodec struct {
	fullname2desc map[string]*MessageDesc
	tof2desc      map[reflect.Type]*MessageDesc
	externalCodec Codec
}

// NewVividCodec 创建并初始化 VividCodec
func NewVividCodec(externalCodec Codec) *VividCodec {
	return &VividCodec{
		fullname2desc: make(map[string]*MessageDesc),
		tof2desc:      make(map[reflect.Type]*MessageDesc),
		externalCodec: externalCodec,
	}
}

func (c *VividCodec) RegisterMessageWithEncoderAndDecoder(class string, name string, message any, encoder MessageEncoder, decoder MessageDecoder) error {
	typeOf := reflect.TypeOf(message)
	messageDesc := newMessageDesc(class, name, typeOf, encoder, decoder)

	if _, ok := c.fullname2desc[messageDesc.FullName()]; ok {
		return fmt.Errorf("message %s already registered", messageDesc.FullName())
	}
	c.fullname2desc[messageDesc.FullName()] = messageDesc
	c.tof2desc[typeOf] = messageDesc
	return nil
}

func (c *VividCodec) RegisterMessage(class string, name string, message MessageCodec) error {
	return c.RegisterMessageWithEncoderAndDecoder(class, name, message, message, message)
}

func (c *VividCodec) FindMessageDesc(class string, name string) *MessageDesc {
	fullname := generateMessageDescFullname(class, name)
	return c.fullname2desc[fullname]
}

func (c *VividCodec) FindMessageDescByType(typeOf reflect.Type) *MessageDesc {
	return c.tof2desc[typeOf]
}

func (c *VividCodec) FindMessageDescByFullname(fullname string) *MessageDesc {
	return c.fullname2desc[fullname]
}

func (c *VividCodec) Encode(writer *Writer, message any) error {
	messageDesc := c.FindMessageDescByType(reflect.TypeOf(message))
	if messageDesc == nil {
		if c.externalCodec == nil {
			return fmt.Errorf("message %s not registered", reflect.TypeOf(message).Name())
		}
		return c.externalCodec.Encode(writer, message)
	}

	// 写入 FULLNAME
	writer.Write(messageDesc.FullName())
	// 写入 MESSAGE_BYTES
	if err := messageDesc.Encode(writer, message); err != nil {
		return err
	}

	return writer.Err()
}

func (c *VividCodec) Decode(reader *Reader) (any, error) {
	var fullname string
	if err := reader.Read(&fullname); err != nil {
		return nil, err
	}

	messageDesc := c.FindMessageDescByFullname(fullname)
	if messageDesc == nil {
		if c.externalCodec == nil {
			return nil, fmt.Errorf("message %s not registered", fullname)
		}
		return c.externalCodec.Decode(reader)
	}

	message := messageDesc.Instantiate()
	if err := messageDesc.Decode(reader, message); err != nil {
		return nil, err
	}

	return message, nil
}
