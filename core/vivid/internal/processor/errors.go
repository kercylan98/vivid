// Package processor 提供了处理单元系统的错误定义。
package processor

import "errors"

// Error 定义了处理器相关错误的类型别名。
// 所有处理器相关的错误都应该使用此类型。
type Error error

var (
	// ErrUnitIdentifierInvalid 表示单元标识符无效错误。
	// 当传入的单元标识符为 nil 或无效时返回此错误。
	ErrUnitIdentifierInvalid = Error(errors.New("nil unit identifier is invalid"))

	// ErrUnitInvalid 表示处理单元无效错误。
	// 当传入的处理单元为 nil 或无效时返回此错误。
	ErrUnitInvalid = Error(errors.New("nil unit is invalid"))

	// ErrUnitAlreadyExists 表示处理单元已存在错误。
	// 当尝试注册一个已存在的处理单元时返回此错误。
	ErrUnitAlreadyExists = Error(errors.New("unit already exists"))

	// ErrUnitNotFound 表示处理单元未找到错误。
	// 当尝试获取一个不存在的处理单元时返回此错误。
	ErrUnitNotFound = Error(errors.New("unit not found"))

	// ErrDaemonUnitNotSet 表示守护单元未设置错误。
	// 当系统没有配置守护单元但尝试获取时返回此错误。
	ErrDaemonUnitNotSet = Error(errors.New("daemon unit not set"))

	// ErrRegistryShutdown 表示注册表已关闭错误。
	// 当注册表已关闭但仍尝试进行操作时返回此错误。
	ErrRegistryShutdown = Error(errors.New("registry has been shutdown"))
)
