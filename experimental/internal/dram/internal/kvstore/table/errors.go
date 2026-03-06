package table

import "errors"

var (
	ErrorNotEnoughSpace = errors.New("not enough space") // 内存不足
	ErrorKeyNotFound    = errors.New("key not found")    // 键不存在
)
