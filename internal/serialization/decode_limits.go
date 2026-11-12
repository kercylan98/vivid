package serialization

import (
	"errors"
	"fmt"
	"math/bits"
)

// ErrDecodeLimit 表示解码超出资源预算，可通过 errors.Is 判断。
var ErrDecodeLimit = errors.New("serialization: decode limit exceeded")

// DecodeLimits 限制同一 Reader 的累计解码资源，零字段使用默认值。
type DecodeLimits struct {
	MaxDepth           uint32 // 复合类型和消息解码的最大嵌套层数
	MaxElements        uint64 // 切片、数组的元素与 map 条目的累计数量，不含字节切片
	MaxAllocationBytes uint64 // 累计申请的内存预算，切片扩容和 map 按估算值计入，不含运行时额外开销
}

// DefaultDecodeLimits 返回默认预算，不共享可修改的全局配置。
func DefaultDecodeLimits() DecodeLimits {
	return DecodeLimits{MaxDepth: 64, MaxElements: 1 << 20, MaxAllocationBytes: 64 << 20}
}

func decodeLimits(options []DecodeLimits) DecodeLimits {
	limits := DefaultDecodeLimits()
	if len(options) == 0 {
		return limits
	}
	option := options[len(options)-1]
	if option.MaxDepth != 0 {
		limits.MaxDepth = option.MaxDepth
	}
	if option.MaxElements != 0 {
		limits.MaxElements = option.MaxElements
	}
	if option.MaxAllocationBytes != 0 {
		limits.MaxAllocationBytes = option.MaxAllocationBytes
	}
	return limits
}

type decodeBudget struct {
	limits          DecodeLimits
	depth           uint32
	elements        uint64
	allocationBytes uint64
}

func (r *Reader) ensureBudget() {
	if r.budget.limits.MaxDepth == 0 {
		r.budget.limits = DefaultDecodeLimits()
	}
}

func (r *Reader) enter() bool {
	if r.err != nil {
		return false
	}
	r.ensureBudget()
	if r.budget.depth >= r.budget.limits.MaxDepth {
		r.limitExceeded("depth", uint64(r.budget.limits.MaxDepth))
		return false
	}
	r.budget.depth++
	return true
}

func (r *Reader) leave() { r.budget.depth-- }

// ReserveElements 为自定义解码器预留集合元素预算，失败后停止此 Reader。
func (r *Reader) ReserveElements(n uint64) error {
	if r.err != nil {
		return r.err
	}
	r.ensureBudget()
	if n > r.budget.limits.MaxElements-r.budget.elements {
		r.limitExceeded("elements", r.budget.limits.MaxElements)
		return r.err
	}
	r.budget.elements += n
	return nil
}

// ReserveAllocation 为自定义解码器自行申请的内存预留预算；Reader 自动管理的分配无需重复预留。
func (r *Reader) ReserveAllocation(n uint64) error {
	if r.checkAllocation(1, n) {
		r.budget.allocationBytes += n
	}
	return r.err
}

func (r *Reader) checkAllocation(count, size uint64) bool {
	if r.err != nil {
		return false
	}
	r.ensureBudget()
	// 检查乘积高位，避免溢出的同时不在热路径执行整数除法。
	high, low := bits.Mul64(count, size)
	if high != 0 || low > r.budget.limits.MaxAllocationBytes-r.budget.allocationBytes {
		r.limitExceeded("allocation bytes", r.budget.limits.MaxAllocationBytes)
		return false
	}
	return true
}

func (r *Reader) limitExceeded(resource string, limit uint64) {
	r.err = fmt.Errorf("%w: %s exceeds %d", ErrDecodeLimit, resource, limit)
}

func (r *Reader) reserveAllocation(count, size uint64) bool {
	if !r.checkAllocation(count, size) {
		return false
	}
	r.budget.allocationBytes += count * size
	return true
}
