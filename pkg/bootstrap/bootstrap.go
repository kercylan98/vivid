package bootstrap

import (
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/actor"
	"github.com/kercylan98/vivid/pkg/result"
)

// NewActorSystem 创建并初始化一个 PrimaryActorSystem 实例。
//
// 参数：
//   - options: 可选配置项（vivid.ActorSystemOption），用于自定义初始化参数（如默认超时、子系统等）
//
// 返回：
//   - *result.Result[vivid.PrimaryActorSystem]：包含 PrimaryActorSystem 实例和错误信息的泛型结果
//
// 说明：
//   - 一般应用仅需一个 ActorSystem 实例。避免将其设为全局变量，以防止潜在的并发问题。
//   - 推荐在主流程（如 main 函数）创建 ActorSystem 及首个 Actor，并通过各 ActorContext 传递和获取 ActorSystem 实例，确保调用顺序清晰、线程安全。
func NewActorSystem(options ...vivid.ActorSystemOption) *result.Result[vivid.PrimaryActorSystem] {
	return result.Cast[*actor.System, vivid.PrimaryActorSystem](actor.NewSystem(options...))
}
