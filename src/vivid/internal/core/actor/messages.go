package actor

import (
	"encoding/gob"
)

func init() {
	gob.RegisterName("vivid.OnKill", new(OnKill))
}

var (
	OnLaunchMessageInstance = new(OnLaunch)
	OnKilledMessageInstance = new(OnKilled)
)

type (
	OnLaunch int8
	OnKill   struct {
		Reason   string // 结束原因
		Operator Ref    // 操作者
		Poison   bool   // 是否为优雅终止
	}
	OnKilled int8
)
