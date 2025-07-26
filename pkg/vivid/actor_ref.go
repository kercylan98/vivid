package vivid

import (
	"github.com/kercylan98/vivid/pkg/vivid/internal/processor"
	"strings"
)

// ActorRef 是 Actor 的引用类型，用于标识和定位 Actor 实例。
//
// ActorRef 提供了 Actor 的地址信息，支持本地和远程 Actor 的统一访问。
// 通过 ActorRef 可以向 Actor 发送消息，而无需直接持有 Actor 实例。
//
// 主要功能：
//   - 唯一标识一个 Actor 实例
//   - 支持跨网络的 Actor 通信
//   - 提供位置透明性
type ActorRef = processor.UnitIdentifier

// NewActorRef 创建一个新的 Actor 引用。
//
// 参数：
//   - address: Actor 所在的地址（如 "localhost:8080"）
//   - path: Actor 在该地址下的路径（如 "/user/actor"）
//
// 返回一个可用于消息发送的 ActorRef 实例。
func NewActorRef(address, path string) ActorRef {
	return processor.NewCacheUnitIdentifier(address, path)
}

// NewActorRefFromAddress 从地址创建一个新的 Actor 引用。
//
// Deprecated: 请使用 ParseActorRef 代替。
func NewActorRefFromAddress(address string) ActorRef {
	var split = strings.SplitN(address, "/", 2)
	return NewActorRef(split[0], split[1])
}

// ParseActorRef 从地址创建一个新的 Actor 引用。
//
// 参数：
//   - address: Actor 的地址（如 "localhost:8080/user/actor"）
//
// 返回一个可用于消息发送的 ActorRef 实例。
func ParseActorRef(address string) ActorRef {
	if address == "" {
		return nil
	}
	var split = strings.SplitN(address, "/", 2)
	return NewActorRef(split[0], split[1])
}
