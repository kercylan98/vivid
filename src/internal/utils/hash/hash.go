package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"time"
)

// GenerateHash 生成一个哈希值
func GenerateHash() string {
	// 获取当前时间和随机数作为种子
	now := time.Now().String()
	randomNumber := rand.Int63()

	// 将当前时间和随机数拼接为一个字符串
	input := fmt.Sprintf("%s%d", now, randomNumber)

	// 使用 sha256 生成哈希
	hash := sha256.New()
	hash.Write([]byte(input))
	hashBytes := hash.Sum(nil)

	// 将哈希值转为十六进制字符串
	hashString := hex.EncodeToString(hashBytes)

	// 截取哈希的前8位作为默认名称
	return hashString[:8]
}
