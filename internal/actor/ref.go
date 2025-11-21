package actor

import (
	"fmt"
	"strings"

	"github.com/kercylan98/vivid"
)

var (
	_ vivid.ActorRef = &Ref{}
)

func NewRef(path vivid.ActorPath) vivid.ActorRef {
	return &Ref{
		path: path,
	}
}

func NewRefWithParent(parent vivid.ActorRef, path vivid.ActorPath) vivid.ActorRef {
	var builder strings.Builder

	builder.WriteString(actorScheme)

	if parent != nil {
		parentPath := strings.TrimRight(parent.GetPath(), "/")
		if parentPath != "" {
			builder.WriteString(parentPath)
		}
	}

	name := strings.TrimLeft(string(path), "/")
	if name == "" {
		name = fmt.Sprint(actorIncrementId.Add(1))
	}
	builder.WriteString("/")
	builder.WriteString(name)

	return NewRef(builder.String())
}

type Ref struct {
	path vivid.ActorPath
}

func (r *Ref) GetPath() vivid.ActorPath {
	return r.path
}

// Tell 是忘却式投递，即不会等待目标 Actor 的响应，也不会返回任何结果。
//
// 参数:
//   - ctx     - 用于投递消息的 ActorContext；
//   - message - 要投递的消息；
func (r *Ref) Tell(ctx vivid.ActorContext, message vivid.Message) {
	ctx.System()
}
