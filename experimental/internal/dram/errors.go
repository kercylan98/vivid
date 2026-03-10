package dram

import "errors"

var (
	ErrShardNotFound = errors.New("shard not found")
	ErrKeyNotFound   = errors.New("key not found")
)
