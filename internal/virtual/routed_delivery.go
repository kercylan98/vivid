package virtual

import (
	"errors"
	"reflect"
	"unicode/utf8"

	"github.com/kercylan98/vivid/internal/serialization"
)

var ErrInvalidRoutedDelivery = errors.New("invalid virtual actor routed delivery")

type DeliveryStatus uint8

const (
	DeliveryAccepted    DeliveryStatus = iota + 1 // 只确认入队，不确认业务处理或副作用完成
	DeliveryStale                                 // 已知归属或代次不匹配
	DeliveryUnavailable                           // 未激活、正在收尾或本次不能确认归属
	DeliveryOverloaded                            // 尚未进入业务邮箱，等待额度已满
	DeliveryStopped
	DeliveryInvalid
)

// RoutedDelivery 绑定一次归属观察；Sender 是业务发送方，外层信封发送方只接收准入回执。
// 组、启动身份与代次用于路由校验，不是认证凭据。
type RoutedDelivery struct {
	Group      [32]byte
	Owner      [16]byte
	Generation uint64
	Kind, Name string
	Sender     string
	Message    any
}

func (m *RoutedDelivery) Valid() bool {
	if m == nil || !m.validHeader() || m.Message == nil {
		return false
	}
	value := reflect.ValueOf(m.Message)
	if value.Kind() == reflect.Pointer && value.IsNil() {
		return false
	}
	_, nested := m.Message.(*RoutedDelivery)
	return !nested
}

func (m *RoutedDelivery) validHeader() bool {
	return m.Group != [32]byte{} && m.Owner != [16]byte{} && m.Generation != 0 && m.Kind != "" && m.Name != "" && len(m.Kind) <= 1024 && len(m.Name) <= 1024 && len(m.Sender) > 0 && len(m.Sender) <= 4<<10 && utf8.ValidString(m.Kind) && utf8.ValidString(m.Name) && utf8.ValidString(m.Sender)
}

// RoutedDeliveryCodec 与业务消息共用原 Reader，嵌套消息不能重置解码预算。
type RoutedDeliveryCodec struct{ codec *serialization.VividCodec }

func NewRoutedDeliveryCodec(codec *serialization.VividCodec) *RoutedDeliveryCodec {
	return &RoutedDeliveryCodec{codec: codec}
}

func (c *RoutedDeliveryCodec) Encode(w *serialization.Writer, message any) error {
	m := message.(*RoutedDelivery)
	if !m.Valid() {
		return ErrInvalidRoutedDelivery
	}
	w.Write(uint8(1), m.Group, m.Owner, m.Generation, m.Kind, m.Name, m.Sender)
	return c.codec.Encode(w, m.Message)
}

func (c *RoutedDeliveryCodec) Decode(r *serialization.Reader, message any) error {
	var m RoutedDelivery
	var version uint8
	if err := r.Read(&version, &m.Group, &m.Owner, &m.Generation, &m.Kind, &m.Name, &m.Sender); err != nil {
		return err
	}
	if version != 1 || !m.validHeader() {
		return ErrInvalidRoutedDelivery
	}
	var err error
	if m.Message, err = c.codec.Decode(r); err != nil {
		return err
	}
	if !m.Valid() {
		return ErrInvalidRoutedDelivery
	}
	*message.(*RoutedDelivery) = m
	return nil
}

type DeliveryResult struct {
	Group      [32]byte
	Owner      [16]byte
	Generation uint64
	Kind, Name string
	Status     DeliveryStatus
}

func (m *DeliveryResult) valid() bool {
	return m.Group != [32]byte{} && m.Owner != [16]byte{} && m.Generation != 0 && m.Kind != "" && m.Name != "" && len(m.Kind) <= 1024 && len(m.Name) <= 1024 && utf8.ValidString(m.Kind) && utf8.ValidString(m.Name) && m.Status >= DeliveryAccepted && m.Status <= DeliveryInvalid
}

func (*DeliveryResult) Encode(w *serialization.Writer, message any) error {
	m := message.(*DeliveryResult)
	if !m.valid() {
		return ErrInvalidRoutedDelivery
	}
	return w.Write(uint8(1), m.Group, m.Owner, m.Generation, m.Kind, m.Name, uint8(m.Status)).Err()
}

func (*DeliveryResult) Decode(r *serialization.Reader, message any) error {
	var m DeliveryResult
	var version uint8
	if err := r.Read(&version, &m.Group, &m.Owner, &m.Generation, &m.Kind, &m.Name, &m.Status); err != nil {
		return err
	}
	if version != 1 || !m.valid() {
		return ErrInvalidRoutedDelivery
	}
	*message.(*DeliveryResult) = m
	return nil
}
