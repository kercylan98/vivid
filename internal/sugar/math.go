package sugar

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
