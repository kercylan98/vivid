package cluster

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// ComputeJoinToken 根据共享密钥与节点状态计算加入认证令牌，供 Join 请求携带。
// 当接收方配置了 JoinSecret 时，请求方须用相同 secret 和 NodeState 调用本函数并填入 AuthToken。
func ComputeJoinToken(secret string, state *NodeState) string {
	if secret == "" || state == nil {
		return ""
	}
	h := hmac.New(sha256.New, []byte(secret))
	_, _ = h.Write([]byte(state.ClusterName))
	_, _ = h.Write([]byte("\n"))
	_, _ = h.Write([]byte(state.ID))
	_, _ = h.Write([]byte("\n"))
	_, _ = h.Write([]byte(state.Address))
	sum := h.Sum(nil)
	return hex.EncodeToString(sum)
}

// VerifyJoinToken 校验 Join 请求的 AuthToken 是否与本地 JoinSecret 及请求中的 NodeState 一致。
func VerifyJoinToken(secret, token string, state *NodeState) bool {
	if secret == "" {
		return true
	}
	expected := ComputeJoinToken(secret, state)
	return len(token) > 0 && hmac.Equal([]byte(expected), []byte(token))
}

const adminTokenPayload = "cluster-admin"

// ComputeAdminToken 根据 AdminSecret 计算管理操作令牌，供强制下线、触发广播等消息携带。
func ComputeAdminToken(secret string) string {
	if secret == "" {
		return ""
	}
	h := hmac.New(sha256.New, []byte(secret))
	_, _ = h.Write([]byte(adminTokenPayload))
	return hex.EncodeToString(h.Sum(nil))
}

// VerifyAdminToken 校验管理操作的 AdminToken。
func VerifyAdminToken(secret, token string) bool {
	if secret == "" {
		return true
	}
	return len(token) > 0 && hmac.Equal([]byte(ComputeAdminToken(secret)), []byte(token))
}
