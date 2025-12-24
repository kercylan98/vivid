package actor_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"

	"github.com/kercylan98/vivid"
)

var _ vivid.Codec = (*TestCodec)(nil)

func NewTestCodec() *TestCodec {
	return &TestCodec{
		name2type: make(map[string]reflect.Type),
		type2name: make(map[reflect.Type]string),
	}
}

type TestCodec struct {
	name2type map[string]reflect.Type
	type2name map[reflect.Type]string
	mu        sync.RWMutex
}

func (c *TestCodec) Register(name string, message vivid.Message) *TestCodec {
	c.mu.Lock()
	defer c.mu.Unlock()

	tof := reflect.TypeOf(message)
	if tof.Kind() != reflect.Ptr {
		panic("message must be a pointer type")
	}

	c.name2type[name] = tof.Elem()
	c.type2name[tof] = name
	return c
}

func (c *TestCodec) Encode(message vivid.Message) ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	tof := reflect.TypeOf(message)
	name, ok := c.type2name[tof]
	if !ok {
		return nil, fmt.Errorf("message type %v not registered", tof)
	}

	// 创建 wrapper 结构
	wrapper := map[string]interface{}{
		"MessageName": name,
		"MessageData": message, // 直接放入消息，不预先编码
	}

	return json.Marshal(wrapper)
}

func (c *TestCodec) Decode(data []byte) (vivid.Message, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 先解码 wrapper
	var wrapper struct {
		MessageName string          `json:"MessageName"`
		MessageData json.RawMessage `json:"MessageData"`
	}

	err := json.Unmarshal(data, &wrapper)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal wrapper: %w", err)
	}

	// 获取消息类型
	typeOf, ok := c.name2type[wrapper.MessageName]
	if !ok {
		return nil, fmt.Errorf("message name %s not found", wrapper.MessageName)
	}

	// 创建消息实例
	messagePtr := reflect.New(typeOf).Interface()

	// 解码消息数据
	err = json.Unmarshal(wrapper.MessageData, messagePtr)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal message data: %w", err)
	}

	// 转换为 vivid.Message
	message, ok := messagePtr.(vivid.Message)
	if !ok {
		return nil, fmt.Errorf("decoded value does not implement vivid.Message")
	}

	return message, nil
}
