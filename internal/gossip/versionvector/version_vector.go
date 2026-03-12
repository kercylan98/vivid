// Package versionvector 实现向量时钟（节点 ID -> 计数），用于 gossip 视图的因果比较与合并。
package versionvector

import (
	"fmt"
	"maps"
	"sort"
	"strings"

	"github.com/kercylan98/vivid/internal/serialization"
)

var (
	_ serialization.MessageCodec = (*VersionVector)(nil)
)

// VersionVector 节点 ID 到版本计数的映射，每次本节点或视图变更时对相应 key 递增，用于 IsBefore/Merge。
type VersionVector struct {
	m map[string]uint64
}

// Decode 从 reader 反序列化，实现 MessageCodec。
func (v *VersionVector) Decode(reader *serialization.Reader, message any) error {
	msg := message.(*VersionVector)
	return reader.Read(&msg.m)
}

// Encode 序列化到 writer，实现 MessageCodec。
func (v *VersionVector) Encode(writer *serialization.Writer, message any) error {
	msg := message.(*VersionVector)
	return writer.Write(msg.m).Err()
}

// New 创建空版本向量。
func New() *VersionVector {
	return &VersionVector{
		m: make(map[string]uint64),
	}
}

// Clone 深拷贝当前向量，用于不可变传递。
func (v *VersionVector) Clone() *VersionVector {
	return &VersionVector{
		m: maps.Clone(v.m),
	}
}

// Increment 将指定 key 的计数加一，状态迁移或本节点视图更新时调用。
func (v *VersionVector) Increment(key string) {
	v.m[key]++
}

// Get 返回指定 key 的版本计数，不存在则返回 0。用于按 key 判断是否接受对方对该成员的更新。
func (v *VersionVector) Get(key string) uint64 {
	if v == nil || v.m == nil {
		return 0
	}
	return v.m[key]
}

// IsBefore 判断当前向量是否严格因果早于 other：对所有 key 有 v[key] <= other[key]，且至少一个 key 严格小于。
// 用于决定是否用对方视图覆盖本地（仅当本地 IsBefore 对方时合并）。
func (v *VersionVector) IsBefore(other *VersionVector) bool {
	if other == nil {
		return false
	}
	foundStrictlyLess := false
	for key, vVal := range v.m {
		oVal, exists := other.m[key]
		if !exists {
			oVal = 0
		}
		if vVal > oVal {
			return false
		}
		if vVal < oVal {
			foundStrictlyLess = true
		}
	}
	for key, oVal := range other.m {
		if _, exists := v.m[key]; !exists {
			if 0 < oVal {
				foundStrictlyLess = true
			} else {
				return false
			}
		}
	}
	return foundStrictlyLess
}

// IsEqual 判断与 other 是否完全相等（所有 key 及值相同）。
func (v *VersionVector) IsEqual(other *VersionVector) bool {
	if other == nil || len(v.m) != len(other.m) {
		return false
	}
	for key, vVal := range v.m {
		if oVal, ok := other.m[key]; !ok || oVal != vVal {
			return false
		}
	}
	return true
}

// Merge 将 other 合并到当前向量：每个 key 取两者较大值，用于收到更新视图后的版本同步。
func (v *VersionVector) Merge(other *VersionVector) {
	if other == nil {
		return
	}
	for key, otherVal := range other.m {
		if currentVal, ok := v.m[key]; !ok || currentVal < otherVal {
			v.m[key] = otherVal
		}
	}
}

// String 返回 map 的字符串表示，便于日志与调试。
func (v *VersionVector) String() string {
	return fmt.Sprintf("%v", v.m)
}

// Fingerprint 返回确定性的视图指纹（按 key 排序），用于收敛检测。
func (v *VersionVector) Fingerprint() string {
	if v == nil || len(v.m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(v.m))
	for k := range v.m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(k)
		b.WriteByte(':')
		b.WriteString(fmt.Sprintf("%d", v.m[k]))
	}
	return b.String()
}

// Remove 从版本向量中移除指定 key。
func (v *VersionVector) Remove(key string) {
	delete(v.m, key)
}
