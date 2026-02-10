package cluster

import "github.com/kercylan98/vivid"

// ActorRefParser 根据集群节点地址与 Actor 路径解析并返回远程 ActorRef。
// 用于在跨节点寻址时，将 address（如 "host:port"）与 path（如 "/user/actorName"）
// 解析为可用的 vivid.ActorRef。若解析失败则返回非 nil 的 error。
type ActorRefParser = func(address string, path string) (vivid.ActorRef, error)
