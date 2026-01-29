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

// Max 返回两个值中的最大值。
func Max[T ~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~float32 | ~float64](a, b T) T {
	if a < b {
		return b
	}
	return a
}

// Min 返回两个值中的最小值。
func Min[T ~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~float32 | ~float64](a, b T) T {
	if a > b {
		return b
	}
	return a
}
