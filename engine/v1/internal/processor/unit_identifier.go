// Package processor 提供了处理单元的注册、管理和路由功能。
// 该包是 Vivid 引擎的核心组件之一，负责处理单元的生命周期管理。
package processor

import (
	"strings"
	"sync/atomic"
)

// NewUnitIdentifier 创建一个新的单元标识符。
// address 参数指定处理单元的网络地址，path 参数指定处理单元的路径。
// 路径会被自动标准化：确保以 "/" 开头，移除末尾的 "/"，处理重复的 "/"。
func NewUnitIdentifier(address string, path string) UnitIdentifier {
	return newUnitIdentifier(address, path)
}

// NewCacheUnitIdentifier 创建一个带缓存功能的单元标识符。
// 带缓存的标识符可以提高重复访问的性能。
func NewCacheUnitIdentifier(address string, path string) CacheUnitIdentifier {
	return newUnitIdentifier(address, path)
}

// newUnitIdentifier 内部构造函数，创建单元标识符实例。
// 该函数会对路径进行标准化处理。
func newUnitIdentifier(address string, path string) *unitIdentifier {
	// 路径标准化处理
	if path == "" {
		path = "/"
	} else if path[0] != '/' {
		path = "/" + path
	}
	if length := len(path); length > 1 && path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}
	path = strings.ReplaceAll(path, "//", "/")

	return &unitIdentifier{
		Address: address,
		Path:    path,
	}
}

// UnitIdentifier 定义了处理单元标识符的基本接口。
// 单元标识符用于唯一标识一个处理单元，包含网络地址和路径信息。
type UnitIdentifier interface {
	// GetAddress 获取处理单元的网络地址
	GetAddress() string

	// GetPath 获取处理单元的路径
	GetPath() string

	// Branch 基于当前标识符生成子单元标识符
	// path 参数将作为子路径追加到当前路径后
	Branch(path string) UnitIdentifier

	// IsRoot 实现 UnitIdentifier 接口，判断标识符是否为根标识符
	IsRoot() bool
}

// CacheUnitIdentifier 定义了带缓存功能的单元标识符接口。
// 继承了 UnitIdentifier 的所有功能，并提供了缓存机制来提高性能。
type CacheUnitIdentifier interface {
	UnitIdentifier

	// GetUnitIdentifier 获取底层的单元标识符
	GetUnitIdentifier() UnitIdentifier

	// LoadCache 从缓存中加载处理单元
	// 如果缓存中没有单元，返回 nil
	LoadCache() Unit

	// StoreCache 将处理单元存储到缓存中
	// unit 参数是要缓存的处理单元
	StoreCache(unit Unit)

	// ClearCache 清除缓存的处理单元
	ClearCache()
}

// unitIdentifier 单元标识符的具体实现。
// 包含地址、路径和原子缓存指针，支持并发安全的缓存操作。
type unitIdentifier struct {
	Address string               // 处理单元地址
	Path    string               // 处理单元路径
	cache   atomic.Pointer[Unit] // 处理单元缓存，使用原子指针保证并发安全
}

// GetAddress 实现 UnitIdentifier 接口，返回处理单元的网络地址。
func (u *unitIdentifier) GetAddress() string {
	return u.Address
}

// GetPath 实现 UnitIdentifier 接口，返回处理单元的路径。
func (u *unitIdentifier) GetPath() string {
	return u.Path
}

// Branch 实现 UnitIdentifier 接口，生成子单元标识符。
// 子路径会被追加到当前路径后，形成新的标识符。
func (u *unitIdentifier) Branch(path string) UnitIdentifier {
	return newUnitIdentifier(u.Address, u.Path+"/"+path)
}

// IsRoot 实现 UnitIdentifier 接口，判断标识符是否为根标识符。
func (u *unitIdentifier) IsRoot() bool {
	return u.Path == "/"
}

// GetUnitIdentifier 实现 CacheUnitIdentifier 接口，返回自身作为基础标识符。
func (u *unitIdentifier) GetUnitIdentifier() UnitIdentifier {
	return u
}

// LoadCache 实现 CacheUnitIdentifier 接口，从缓存中加载处理单元。
// 使用原子操作保证并发安全，如果缓存为空则返回 nil。
func (u *unitIdentifier) LoadCache() Unit {
	if cache := u.cache.Load(); cache != nil {
		return *cache
	}
	return nil
}

// StoreCache 实现 CacheUnitIdentifier 接口，将处理单元存储到缓存中。
// 使用原子操作保证并发安全。
func (u *unitIdentifier) StoreCache(unit Unit) {
	u.cache.Store(&unit)
}

// ClearCache 实现 CacheUnitIdentifier 接口，清除缓存的处理单元。
// 使用原子操作保证并发安全。
func (u *unitIdentifier) ClearCache() {
	u.cache.Store(nil)
}
