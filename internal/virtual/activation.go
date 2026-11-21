package virtual

import (
	"errors"
	"time"
	"unicode/utf8"

	"github.com/kercylan98/vivid/internal/serialization"
)

var ErrInvalidActivation = errors.New("invalid virtual actor activation")

type ActivationStatus uint8

const (
	ActivationReady ActivationStatus = iota + 1 // 预启动与邮箱初始化完成且当次归属复核成功，不等待 OnLaunch 业务处理
	ActivationConflict
	ActivationUnavailable
	ActivationOverloaded
	ActivationStopped
	ActivationInvalid
	ActivationFailed
	ActivationUnknown // 取得归属的结果未知，不得换节点重新认领
)

// ActivationRequest 不包含业务消息。Generation 为 0 时才允许尝试取得不存在的归属。
// Owner 绑定目标启动实例，同地址重启不能接纳发给旧实例的激活请求。
type ActivationRequest struct {
	Group      [32]byte
	Owner      [16]byte
	Generation uint64
	Kind, Name string
	Timeout    time.Duration
}

func (m *ActivationRequest) Valid() bool {
	return m != nil && validActivationIdentity(m.Group, m.Owner, m.Kind, m.Name) && m.Timeout > 0
}

func validActivationIdentity(group [32]byte, owner [16]byte, kind, name string) bool {
	return group != [32]byte{} && owner != [16]byte{} && kind != "" && name != "" && len(kind) <= 1024 && len(name) <= 1024 && utf8.ValidString(kind) && utf8.ValidString(name)
}

func (*ActivationRequest) Encode(w *serialization.Writer, message any) error {
	m := message.(*ActivationRequest)
	if !m.Valid() {
		return ErrInvalidActivation
	}
	return w.Write(uint8(1), m.Group, m.Owner, m.Generation, m.Kind, m.Name, m.Timeout).Err()
}

func (*ActivationRequest) Decode(r *serialization.Reader, message any) error {
	var m ActivationRequest
	var version uint8
	if err := r.Read(&version, &m.Group, &m.Owner, &m.Generation, &m.Kind, &m.Name, &m.Timeout); err != nil {
		return err
	}
	if version != 1 || !m.Valid() {
		return ErrInvalidActivation
	}
	*message.(*ActivationRequest) = m
	return nil
}

type ActivationResult struct {
	Group             [32]byte
	Owner             [16]byte
	Generation        uint64 // 回显请求中的预期代次
	Kind, Name        string
	Status            ActivationStatus
	CurrentGeneration uint64 // 仅 Ready 回执携带实际代次
}

func (m *ActivationResult) Valid() bool {
	if m == nil || !validActivationIdentity(m.Group, m.Owner, m.Kind, m.Name) || m.Status < ActivationReady || m.Status > ActivationUnknown {
		return false
	}
	if m.Status == ActivationReady {
		return m.CurrentGeneration != 0 && (m.Generation == 0 || m.Generation == m.CurrentGeneration)
	}
	return m.CurrentGeneration == 0
}

func (*ActivationResult) Encode(w *serialization.Writer, message any) error {
	m := message.(*ActivationResult)
	if !m.Valid() {
		return ErrInvalidActivation
	}
	return w.Write(uint8(1), m.Group, m.Owner, m.Generation, m.Kind, m.Name, uint8(m.Status), m.CurrentGeneration).Err()
}

func (*ActivationResult) Decode(r *serialization.Reader, message any) error {
	var m ActivationResult
	var version uint8
	if err := r.Read(&version, &m.Group, &m.Owner, &m.Generation, &m.Kind, &m.Name, &m.Status, &m.CurrentGeneration); err != nil {
		return err
	}
	if version != 1 || !m.Valid() {
		return ErrInvalidActivation
	}
	*message.(*ActivationResult) = m
	return nil
}
