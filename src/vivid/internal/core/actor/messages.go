package actor

import (
	"encoding/gob"
)

func init() {
	gob.RegisterName("vivid.OnKill", new(OnKill))
	gob.RegisterName("vivid.OnKilled", new(OnKilled))
	gob.RegisterName("vivid.OnWatch", new(OnWatch))
	gob.RegisterName("vivid.OnUnwatch", new(OnUnwatch))
	gob.RegisterName("vivid.OnDead", new(OnDead))
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
)

func (o *OnLaunch) Restarted() bool {
	return o == OnRestartMessageInstance
}
