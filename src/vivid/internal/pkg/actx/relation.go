package actx

import (
	"github.com/kercylan98/vivid/src/vivid/internal/core"
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
)

var _ actor.RelationContext = (*Relation)(nil)

type Relation struct {
	guid     uint64                  // 子 Actor Guid 计数器
	children map[core.Path]actor.Ref // 子 Actor 集合（懒加载）
}

func (r *Relation) NextGuid() uint64 {
	r.guid++
	return r.guid
}

func (r *Relation) BindChild(child actor.Ref) {
	if r.children == nil {
		r.children = make(map[core.Path]actor.Ref)
	}
	r.children[child.Path()] = child
}

func (r *Relation) UnbindChild(child actor.Ref) {
	if _, found := r.children[child.Path()]; found {
		delete(r.children, child.Path())
		if len(r.children) == 0 {
			r.children = nil
		}
	}
}
