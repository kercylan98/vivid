package actx

import (
	"github.com/kercylan98/go-log/log"
	"github.com/kercylan98/vivid/src/vivid/internal/core"
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
)

var _ actor.RelationContext = (*Relation)(nil)

func NewRelation(ctx *Context) *Relation {
	return &Relation{
		ctx: ctx,
	}
}

type Relation struct {
	ctx      *Context
	guid     int64                   // 子 Actor Guid 计数器
	children map[core.Path]actor.Ref // 子 Actor 集合（懒加载）
	watcher  map[core.URL]actor.Ref  // 监视者集合
}

func (r *Relation) ResetWatchers() {
	r.watcher = nil
}

func (r *Relation) Watchers() map[core.URL]actor.Ref {
	return r.watcher
}

func (r *Relation) Watch(ref actor.Ref) {
	r.ctx.TransportContext().Probe(ref, SystemMessage, actor.OnWatchMessageInstance)
}

func (r *Relation) Unwatch(ref actor.Ref) {
	r.ctx.TransportContext().Probe(ref, SystemMessage, actor.OnUnwatchMessageInstance)
}

func (r *Relation) AddWatcher(ref actor.Ref) {
	// 如果当前 Actor 已经终止，则不再添加监视者，并且立即告知监视者
	meta := r.ctx.MetadataContext()
	if r.ctx.LifecycleContext().Status() == lifecycleStatusTerminated {
		r.ctx.TransportContext().Reply(UserMessage, &actor.OnDead{
			Ref: meta.Ref(),
		})
		return
	}

	url := ref.URL()
	if _, found := r.watcher[url]; found {
		return
	}

	if r.watcher == nil {
		r.watcher = make(map[core.URL]actor.Ref)
	}

	r.watcher[url] = ref
	meta.Config().LoggerProvider.Provide().Debug("watcher", log.String("ref", meta.Ref().Path()), log.String("add", ref.URL()))
}

func (r *Relation) RemoveWatcher(ref actor.Ref) {
	url := ref.URL()
	if _, found := r.watcher[url]; found {
		delete(r.watcher, url)
		if len(r.watcher) == 0 {
			r.watcher = nil
		}
		meta := r.ctx.MetadataContext()
		meta.Config().LoggerProvider.Provide().Debug("watcher", log.String("ref", meta.Ref().Path()), log.String("remove", ref.URL()))
	}
}

func (r *Relation) Children() map[core.Path]actor.Ref {
	return r.children
}

func (r *Relation) NextGuid() int64 {
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
