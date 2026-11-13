package utils_test

import (
	"testing"

	"github.com/kercylan98/vivid/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeRemoteAddress(t *testing.T) {
	for _, address := range []string{"127.0.0.1:1", "node.example:65535", "localhost:8080", "[::1]:8080", "[2001:db8::1]:443", "[::ffff:127.0.0.1]:8080", "node:00080"} {
		t.Run(address, func(t *testing.T) {
			normalized, ok := utils.NormalizeRemoteAddress(" \t" + address + "\n")
			assert.True(t, ok)
			assert.Equal(t, address, normalized, "只裁剪首尾空白，不改变身份字符串")
		})
	}
	for _, address := range []string{"", " ", "node.example", "127.0.0.1", "::1", ":8080", "0.0.0.0:8080", "[::]:8080", "[0:0:0:0:0:0:0:0]:8080", "[::ffff:0.0.0.0]:8080", "node:0", "node:65536", "node:-1", "node:+80", "node:http", "node:", "bad host:80", "node: 80", "::1:8080"} {
		t.Run(address, func(t *testing.T) {
			normalized, ok := utils.NormalizeRemoteAddress(address)
			assert.False(t, ok)
			assert.Empty(t, normalized)
		})
	}
}
