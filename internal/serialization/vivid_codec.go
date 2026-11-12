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
	fullName2desc map[string]*MessageDesc
	tof2desc      map[reflect.Type]*MessageDesc
	externalCodec Codec
	decodeLimits  DecodeLimits
}

// NewVividCodec 创建并初始化 VividCodec
func NewVividCodec(externalCodec Codec, limits ...DecodeLimits) *VividCodec {
	return &VividCodec{
		fullName2desc: make(map[string]*MessageDesc),
		tof2desc:      make(map[reflect.Type]*MessageDesc),
		externalCodec: externalCodec,
		decodeLimits:  decodeLimits(limits),
	}
}

// DecodeLimits 返回远程信封 Reader 使用的解码预算，独立 Reader 保留自身配置。
func (c *VividCodec) DecodeLimits() DecodeLimits { return c.decodeLimits }

func (c *VividCodec) RegisterMessageWithEncoderAndDecoder(class string, name string, message any, encoder MessageEncoder, decoder MessageDecoder) error {
	typeOf := reflect.TypeOf(message)
	messageDesc := newMessageDesc(class, name, typeOf, encoder, decoder)

	if _, ok := c.fullName2desc[messageDesc.FullName()]; ok {
		return fmt.Errorf("message %s already registered", messageDesc.FullName())
	}
	c.fullName2desc[messageDesc.FullName()] = messageDesc
	c.tof2desc[typeOf] = messageDesc
	return nil
}

func (c *VividCodec) RegisterMessage(class string, name string, message MessageCodec) error {
	return c.RegisterMessageWithEncoderAndDecoder(class, name, message, message, message)
}

func (c *VividCodec) FindMessageDesc(class string, name string) *MessageDesc {
	fullName := generateMessageDescFullName(class, name)
	return c.fullName2desc[fullName]
}

func (c *VividCodec) FindMessageDescByType(typeOf reflect.Type) *MessageDesc {
	return c.tof2desc[typeOf]
}

func (c *VividCodec) FindMessageDescByFullName(fullName string) *MessageDesc {
	return c.fullName2desc[fullName]
}

func (c *VividCodec) Encode(writer *Writer, message any) error {
	if err := writer.Err(); err != nil {
		return err
	}
	typ := reflect.TypeOf(message)
	if typ == nil || typ.Kind() == reflect.Ptr && reflect.ValueOf(message).IsNil() {
		return fmt.Errorf("serialization: cannot Encode nil message")
	}
	messageDesc := c.FindMessageDescByType(typ)
	if messageDesc == nil {
		if c.externalCodec == nil {
			return fmt.Errorf("message %s not registered", reflect.TypeOf(message).Name())
		}
		// 空名称标识外部 Codec，消息分帧由框架统一负责。
		writer.WriteString("")
		if err := c.externalCodec.Encode(writer, message); err != nil {
			return err
		}
		return writer.Err()
	}

	// 写入 FULL_NAME
	writer.WriteString(messageDesc.FullName())
	// 写入 MESSAGE_BYTES
	if err := messageDesc.Encode(writer, message); err != nil {
		return err
	}

	return writer.Err()
}

func (c *VividCodec) Decode(reader *Reader) (any, error) {
	if !reader.enter() {
		return nil, reader.Err()
	}
	defer reader.leave()
	var fullName string
	if err := reader.ReadString(&fullName); err != nil {
		return nil, err
	}

	if fullName == "" && c.externalCodec != nil {
		message, err := c.externalCodec.Decode(reader)
		if err == nil {
			err = reader.Err()
		}
		return message, err
	}
	messageDesc := c.FindMessageDescByFullName(fullName)
	if messageDesc == nil {
		return nil, fmt.Errorf("message %s not registered", fullName)
	}

	typ := messageDesc.typeOf
	count := uint64(2) // 非指针类型还需复制到接口。
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
		count = 1
	}
	if !reader.reserveAllocation(count, uint64(typ.Size())) {
		return nil, reader.Err()
	}
	message := messageDesc.Instantiate()
	if err := messageDesc.Decode(reader, message); err != nil {
		return nil, err
	}

	if err := reader.Err(); err != nil {
		return nil, err
	}
	return message, nil
}
