// Package sugar 提供了一组便捷的辅助工具函数，用于简化日常开发中的常见操作。
// 该包与传统的 utils 包相区分，侧重于提供与切片、映射、通用类型处理等相关的快捷工具函数，
// 以提升开发效率、减少样板代码，增强代码可读性与可维护性。
// 所有函数均为独立工具函数，不依赖具体业务逻辑，可广泛复用于各类场景。
package sugar

// FirstOrDefault 返回切片中的第一个元素，如果切片为空，则返回默认值。
func FirstOrDefault[T any](slice []T, defaultValue T) T {
	if len(slice) == 0 {
		return defaultValue
	}
	return slice[0]
}
