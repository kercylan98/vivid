package vivid

import "strings"

// ActorRef 定义了 Actor 的抽象引用类型，作为唯一标识和操作 Actor 实例的基本句柄。
//
// ActorRef 主要用于分布式和本地环境下消息投递、身份判断、路径定位及创建 Actor 引用拷贝等场景。
//
// 实现说明：
//   - 框架保证每个 ActorRef 实例均与唯一的 Actor 绑定，引用等价性由 Equals 方法保证。
//   - ActorRef 是线程安全的，推荐在多协程环境下广泛传递与复用。
//   - 不同实现可代表本地、本地代理或远程 Actor，调用方无需关心具体实现细节。
type ActorRef interface {
	// GetAddress 返回该 Actor 所属系统或节点的地址（例如 IP:端口、集群节点标识等）。
	//
	// 主要用于分布式场景下区分不同主机或节点上的 Actor 所在位置。
	GetAddress() string

	// GetPath 返回该 Actor 在所属系统下的唯一路径 ActorPath。
	//
	// 路径由父子关系与命名组成，用于唯一定位同一节点或系统内的 Actor 层级。
	GetPath() ActorPath

	// Equals 判断当前 ActorRef 是否与另一个 ActorRef 语义等价。
	//
	// 等价性依据实现，通常比较地址与路径，便于做集合去重、哈希表索引等。
	// 若比较对象为 nil 或类型不同，通常返回 false。
	Equals(other ActorRef) bool

	// Clone 返回当前 ActorRef 的独立副本，内容等价但实例化为新对象。
	//
	// 用于缓存、并发传递等需要 ActorRef 不受外部影响的场景。
	Clone() ActorRef

	// ToActorRefs 返回当前 ActorRef 的切片，用于将 ActorRef 转换为切片。
	ToActorRefs() ActorRefs
}

// ActorRefs 定义了 ActorRef 的切片类型，表示一组 ActorRef。
type ActorRefs []ActorRef

// ToSlice 返回当前 ActorRefs 的切片，用于将 ActorRefs 转换为切片。
func (refs ActorRefs) ToSlice() []ActorRef {
	return refs
}

// First 返回当前 ActorRefs 的第一个 ActorRef。
func (refs ActorRefs) First() ActorRef {
	if len(refs) == 0 {
		return nil
	}
	return refs[0]
}

// Last 返回当前 ActorRefs 的最后一个 ActorRef。
func (refs ActorRefs) Last() ActorRef {
	if len(refs) == 0 {
		return nil
	}
	return refs[len(refs)-1]
}

// Len 返回当前 ActorRefs 的长度。
func (refs ActorRefs) Len() int {
	return len(refs)
}

// Empty 返回当前 ActorRefs 是否为空。
func (refs ActorRefs) Empty() bool {
	return len(refs) == 0
}

// Contains 返回当前 ActorRefs 是否包含指定的 ActorRef。
func (refs ActorRefs) Contains(ref ActorRef) bool {
	for _, r := range refs {
		if r.Equals(ref) {
			return true
		}
	}
	return false
}

// Index 返回当前 ActorRefs 中指定 ActorRef 的索引。

func (refs ActorRefs) Index(ref ActorRef) int {
	for i, r := range refs {
		if r.Equals(ref) {
			return i
		}
	}
	return -1
}

// Remove 返回当前 ActorRefs 中指定 ActorRef 的切片，用于去除指定的 ActorRef。
func (refs ActorRefs) Remove(ref ActorRef) ActorRefs {
	var result ActorRefs
	for _, r := range refs {
		if !r.Equals(ref) {
			result = append(result, r)
		}
	}
	return result
}

// RemoveAt 返回当前 ActorRefs 中指定索引的切片，用于去除指定的 ActorRef。
func (refs ActorRefs) RemoveAt(index int) ActorRefs {
	return append(refs[:index], refs[index+1:]...)
}

// RemoveMany 返回当前 ActorRefs 中指定索引的切片，用于去除指定的 ActorRef。
func (refs ActorRefs) RemoveMany(indices ...int) ActorRefs {
	var result ActorRefs
	for _, index := range indices {
		result = append(result, refs[index])
	}
	return result
}

// Unique 返回当前 ActorRefs 的唯一切片，用于去除重复的 ActorRef。
func (refs ActorRefs) Unique() ActorRefs {
	unique := make(map[string]struct{})
	var result ActorRefs
	for _, ref := range refs {
		addr, path := ref.GetAddress(), ref.GetPath()
		concat := addr + path
		if _, ok := unique[concat]; !ok {
			unique[concat] = struct{}{}
			result = append(result, ref)
		}
	}
	return result
}

// Iterator 返回当前 ActorRefs 的迭代器，用于遍历 ActorRefs。
// backwards 为 true 时，从后往前遍历，否则从前往后遍历。
func (refs ActorRefs) Iterator(backwards bool) func(yield func(ActorRef) bool) {
	if backwards {
		return func(yield func(ActorRef) bool) {
			for i := len(refs) - 1; i >= 0; i-- {
				if !yield(refs[i]) {
					break
				}
			}
		}
	}
	return func(yield func(ActorRef) bool) {
		for _, ref := range refs {
			if !yield(ref) {
				break
			}
		}
	}
}

// Filter 返回当前 ActorRefs 的切片，用于过滤符合条件的 ActorRef。
// fn 为 true 时，保留该 ActorRef，否则去除该 ActorRef。
func (refs ActorRefs) Filter(fn func(ActorRef) bool) ActorRefs {
	var result ActorRefs
	for _, ref := range refs {
		if fn(ref) {
			result = append(result, ref)
		}
	}
	return result
}

// FormSlice 从切片中创建 ActorRefs。
func (refs ActorRefs) FromSlice(slice []ActorRef) ActorRefs {
	return slice
}

// Combine 合并两个 ActorRefs。
func (refs ActorRefs) Combine(other ActorRefs) ActorRefs {
	return append(refs, other...)
}

// Intersect 返回当前 ActorRefs 和另一个 ActorRefs 的交集。
func (refs ActorRefs) Intersect(other ActorRefs) ActorRefs {
	return refs.Filter(func(ref ActorRef) bool {
		return other.Contains(ref)
	})
}

// Union 返回当前 ActorRefs 和另一个 ActorRefs 的并集。
func (refs ActorRefs) Union(other ActorRefs) ActorRefs {
	return refs.Combine(other).Unique()
}

// Difference 返回当前 ActorRefs 和另一个 ActorRefs 的差集。
func (refs ActorRefs) Difference(other ActorRefs) ActorRefs {
	return refs.Filter(func(ref ActorRef) bool {
		return !other.Contains(ref)
	})
}

// Clone 返回当前 ActorRefs 的副本。
func (refs ActorRefs) Clone() ActorRefs {
	var result = make(ActorRefs, len(refs))
	copy(result, refs)
	return result
}

// DeepClone 返回当前 ActorRefs 的深度副本。
// 深度副本会递归克隆每个 ActorRef。
func (refs ActorRefs) DeepClone() ActorRefs {
	var result = make(ActorRefs, len(refs))
	for i, ref := range refs {
		result[i] = ref.Clone()
	}
	return result
}

// String 返回当前 ActorRefs 的字符串表示。
func (refs ActorRefs) String() string {
	var result strings.Builder
	result.WriteString("[")
	for _, ref := range refs {
		result.WriteString(ref.GetAddress() + "/" + ref.GetPath())
		result.WriteString(", ")
	}
	result.WriteString("]")
	return strings.TrimSuffix(result.String(), ", ")
}
