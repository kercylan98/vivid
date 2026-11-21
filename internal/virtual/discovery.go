package virtual

import (
	"errors"
	"unicode/utf8"

	"github.com/kercylan98/vivid/internal/serialization"
)

var ErrInvalidDiscovery = errors.New("invalid virtual actor discovery")

type DiscoveryStatus uint8

const (
	DiscoveryAvailable DiscoveryStatus = iota + 1 // 仅提供当前启动身份，不表示归属或激活就绪
	DiscoveryUnavailable
	DiscoveryStopped
	DiscoveryInvalid
)

// DiscoveryRequest 只查询固定入口是否支持该 kind，不包含业务消息或归属认领。
type DiscoveryRequest struct {
	Group [32]byte
	Kind  string
}

func (m *DiscoveryRequest) Valid() bool {
	return m != nil && validDiscoveryIdentity(m.Group, m.Kind)
}

func validDiscoveryIdentity(group [32]byte, kind string) bool {
	return group != [32]byte{} && kind != "" && len(kind) <= 1024 && utf8.ValidString(kind)
}

func (*DiscoveryRequest) Encode(w *serialization.Writer, message any) error {
	m := message.(*DiscoveryRequest)
	if !m.Valid() {
		return ErrInvalidDiscovery
	}
	return w.Write(uint8(1), m.Group, m.Kind).Err()
}

func (*DiscoveryRequest) Decode(r *serialization.Reader, message any) error {
	var m DiscoveryRequest
	var version uint8
	if err := r.Read(&version, &m.Group, &m.Kind); err != nil {
		return err
	}
	if version != 1 || !m.Valid() {
		return ErrInvalidDiscovery
	}
	*message.(*DiscoveryRequest) = m
	return nil
}

// DiscoveryResult 不返回可重定向地址；调用方只联系自己配置的入口。
type DiscoveryResult struct {
	Group  [32]byte
	Kind   string
	Status DiscoveryStatus
	Owner  [16]byte // 仅 Available 携带当前启动身份
}

func (m *DiscoveryResult) Valid() bool {
	if m == nil || !validDiscoveryIdentity(m.Group, m.Kind) || m.Status < DiscoveryAvailable || m.Status > DiscoveryInvalid {
		return false
	}
	return (m.Status == DiscoveryAvailable) == (m.Owner != [16]byte{})
}

func (*DiscoveryResult) Encode(w *serialization.Writer, message any) error {
	m := message.(*DiscoveryResult)
	if !m.Valid() {
		return ErrInvalidDiscovery
	}
	return w.Write(uint8(1), m.Group, m.Kind, uint8(m.Status), m.Owner).Err()
}

func (*DiscoveryResult) Decode(r *serialization.Reader, message any) error {
	var m DiscoveryResult
	var version uint8
	if err := r.Read(&version, &m.Group, &m.Kind, &m.Status, &m.Owner); err != nil {
		return err
	}
	if version != 1 || !m.Valid() {
		return ErrInvalidDiscovery
	}
	*message.(*DiscoveryResult) = m
	return nil
}
