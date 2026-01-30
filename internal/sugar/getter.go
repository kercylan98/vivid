package sugar

// GetOrDefault 返回非 nil 的值，如果值为 nil，则返回默认值。
func GetOrDefault[T *any](value T, defaultValue T) T {
	if value == nil {
		return defaultValue
	}
	return value
}
