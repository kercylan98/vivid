package versionvector

import (
	"fmt"
	"maps"

	"github.com/kercylan98/vivid/internal/messages"
)

func init() {
	messages.RegisterInternalMessage[*VersionVector]("VersionVector", onVersionVectorReader, onVersionVectorWriter)
}

// VersionVector 版本向量结构体，保存节点id到计数器的映射
type VersionVector struct {
	m map[string]uint64
}

func onVersionVectorReader(message any, reader *messages.Reader, codec messages.Codec) error {
	v := message.(*VersionVector)
	if v.m == nil {
		v.m = make(map[string]uint64)
	}

	return reader.Read(&v.m)
}

func onVersionVectorWriter(message any, writer *messages.Writer, codec messages.Codec) error {
	v := message.(*VersionVector)
	writer.Write(v.m)
	return writer.Err()
}

// New 创建一个新的版本向量
func New() *VersionVector {
	return &VersionVector{
		m: make(map[string]uint64),
	}
}

// Clone 创建一个当前版本向量的深拷贝
func (v *VersionVector) Clone() *VersionVector {
	return &VersionVector{
		m: maps.Clone(v.m),
	}
}

// Increment 对指定 key 的计数器加一
func (v *VersionVector) Increment(key string) {
	v.m[key]++
}

// IsBefore 判断当前版本向量是否严格早于另一个版本向量
// 返回true当且仅当：对于所有key，v[key] ≤ other[key]，且至少存在一个key使得v[key] < other[key]
func (v *VersionVector) IsBefore(other *VersionVector) bool {
	if other == nil {
		return false
	}

	foundStrictlyLess := false

	// 检查v中所有key
	for key, vVal := range v.m {
		oVal, exists := other.m[key]
		if !exists {
			oVal = 0 // other中没有的key视为0
		}

		if vVal > oVal {
			return false // v有更大的值，不可能早于other
		}
		if vVal < oVal {
			foundStrictlyLess = true
		}
	}

	// 检查other中有而v中没有的key
	for key, oVal := range other.m {
		if _, exists := v.m[key]; !exists {
			// v中没有这个key，视为v[key] = 0
			if 0 < oVal {
				foundStrictlyLess = true // v[key]=0 < other[key]
			} else {
				return false // 不可能发生，因为oVal ≥ 0
			}
		}
	}

	return foundStrictlyLess
}

// IsEqual 判断两个版本向量是否完全相同
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

// Merge 将另一个版本向量合并到当前向量，采用每个 key 的最大值
func (v *VersionVector) Merge(other *VersionVector) {
	if other == nil {
		return // 兼容 nil
	}
	for key, otherVal := range other.m {
		if currentVal, ok := v.m[key]; !ok || currentVal < otherVal {
			v.m[key] = otherVal
		}
	}
}

// String 返回版本向量的字符串表示
func (v *VersionVector) String() string {
	return fmt.Sprintf("%v", v.m)
}
