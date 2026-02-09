package cluster

import (
	"time"

	"github.com/kercylan98/vivid"
)

// JoinAskTimeout 返回 Join 请求使用的 Ask 超时，1s–30s 限幅；未配置时返回 5s。
func JoinAskTimeout(opts vivid.ClusterOptions) time.Duration {
	d := opts.JoinAskTimeout
	if d <= 0 {
		return 5 * time.Second
	}
	if d < time.Second {
		return time.Second
	}
	if d > 30*time.Second {
		return 30 * time.Second
	}
	return d
}

// GetViewAskTimeout 返回 GetView 请求使用的 Ask 超时，1s–30s 限幅；未配置时返回 5s。
func GetViewAskTimeout(opts vivid.ClusterOptions) time.Duration {
	d := opts.GetViewAskTimeout
	if d <= 0 {
		return 5 * time.Second
	}
	if d < time.Second {
		return time.Second
	}
	if d > 30*time.Second {
		return 30 * time.Second
	}
	return d
}
