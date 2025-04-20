package actor

import (
	"encoding/gob"
	"time"
)

func init() {
	gob.RegisterName("vivid.OnKill", new(OnKill))
	gob.RegisterName("vivid.OnKilled", new(OnKilled))
	gob.RegisterName("vivid.OnWatch", new(OnWatch))
	gob.RegisterName("vivid.OnUnwatch", new(OnUnwatch))
	gob.RegisterName("vivid.OnDead", new(OnDead))
	gob.RegisterName("vivid.OnPing", new(OnPing))
	gob.RegisterName("vivid.OnPong", new(OnPong))
}

var (
	OnLaunchMessageInstance  = new(OnLaunch)
	OnRestartMessageInstance = new(OnLaunch)
	OnWatchMessageInstance   = new(OnWatch)
	OnUnwatchMessageInstance = new(OnUnwatch)
)

type (
	OnLaunch int8
	OnKill   struct {
		Reason   string // 结束原因
		Operator Ref    // 操作者
		Poison   bool   // 是否为优雅终止
		Restart  bool   // 是否需要终止后重启
	}
	OnKilled  OnKill
	OnWatch   int8
	OnUnwatch int8
	OnDead    struct {
		Ref Ref // 生命周期结束的 Actor
	}
	OnPing struct {
		Timestamp int64 // 发送时间戳
	}
	OnPong struct {
		OriginalTimestamp int64 // 原始Ping的时间戳
		Timestamp         int64 // 响应时间戳
	}
)

func (o *OnLaunch) Restarted() bool {
	return o == OnRestartMessageInstance
}

// RTT 返回往返时间
func (p *OnPong) RTT() time.Duration {
	return time.Duration(p.Timestamp - p.OriginalTimestamp)
}

// OriginalTime 返回原始 Ping 的时间
func (p *OnPong) OriginalTime() time.Time {
	return time.Unix(0, p.OriginalTimestamp)
}

// ResponseTime 返回响应时间
func (p *OnPong) ResponseTime() time.Time {
	return time.Unix(0, p.Timestamp)
}
