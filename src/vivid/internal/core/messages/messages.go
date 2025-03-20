package messages

import (
	"encoding/gob"
	"github.com/kercylan98/vivid/src/vivid/internal/core/actor"
)

func init() {
	gob.RegisterName("vivid.OnKill", new(OnKill))
}

var (
	OnLaunchMessageInstance = new(OnLaunch)
)

type (
	OnLaunch int8
	OnKill   struct {
		Reason   string    // 结束原因
		Operator actor.Ref // 操作者
		Poison   bool      // 是否为优雅终止
	}
)
