package vividkit

import (
	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/actor"
)

// NewActorRef 创建一个新的 ActorRef 实例。
// 参数：
//   - address: actor 所在的地址（如主机名、域名或IP:端口等）。
//   - path: actor 的路径（格式须遵循系统规范，如 "/user/actor1"）。
//
// 返回值：
//   - vivid.ActorRef: 生成的 actor 引用对象，如果参数非法，则为 nil。
//   - error: 地址或路径不合法时返回详细错误信息。
//
// 该函数包装了 internal/actor.NewRef，推荐作为外部包构建 actor 引用的统一入口。
func NewActorRef(address, path string) (vivid.ActorRef, error) {
	return actor.NewRef(address, path)
}

// ParseActorRef 依据字符串解析生成 ActorRef 实例。
// 参数：
//   - ref: actor 引用字符串（如 "example.com:8080/user/a"，支持两种格式，详细参见 internal 文档）。
//
// 返回值：
//   - vivid.ActorRef: 解析得到的 actor 引用对象，若解析失败则为 nil。
//   - error: 字符串格式、地址或路径非法时返回对应错误。
//
// 用于把存储、传输的字符串形式 actor ref 转为可用的 ActorRef 对象。
func ParseActorRef(ref string) (vivid.ActorRef, error) {
	return actor.ParseRef(ref)
}
