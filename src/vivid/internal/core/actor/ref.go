package actor

import "github.com/kercylan98/vivid/src/vivid/internal/core"

type Ref interface {
	// Address 返回 Ref 的地址
	Address() core.Address

	// Path 返回 Ref 的路径
	Path() core.Path

	// Equal 判断两个 Ref 是否相等
	Equal(ref Ref) bool

	// GenerateSub 生成一个子 Ref
	GenerateSub(path core.Path) Ref
}
