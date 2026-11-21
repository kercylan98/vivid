package vivid_test

import (
	"strings"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/stretchr/testify/require"
)

func TestCoordinationOptions_ValidateAndSnapshot(t *testing.T) {
	valid := vivid.ActorSystemCoordinationOptions{GroupID: "owners", NodeID: 1, Members: map[uint64]string{1: "127.0.0.1:9000"}, Path: "node.wal"}
	require.NoError(t, valid.Validate())
	require.Zero(t, valid.HeartbeatInterval, "校验不修改调用方默认值")
	for _, change := range []func(*vivid.ActorSystemCoordinationOptions){
		func(o *vivid.ActorSystemCoordinationOptions) { o.GroupID = "" },
		func(o *vivid.ActorSystemCoordinationOptions) { o.GroupID = strings.Repeat("a", 129) },
		func(o *vivid.ActorSystemCoordinationOptions) { o.GroupID = "\xff" },
		func(o *vivid.ActorSystemCoordinationOptions) { o.NodeID = 0 },
		func(o *vivid.ActorSystemCoordinationOptions) { o.NodeID = 2 },
		func(o *vivid.ActorSystemCoordinationOptions) { o.Path = "" },
		func(o *vivid.ActorSystemCoordinationOptions) { o.Members = nil },
		func(o *vivid.ActorSystemCoordinationOptions) { o.Members = map[uint64]string{0: "127.0.0.1:9000"} },
		func(o *vivid.ActorSystemCoordinationOptions) { o.Members = map[uint64]string{1: "0.0.0.0:9000"} },
		func(o *vivid.ActorSystemCoordinationOptions) {
			o.Members = map[uint64]string{1: "127.0.0.1:9000", 2: " 127.0.0.1:9000 "}
		},
		func(o *vivid.ActorSystemCoordinationOptions) { o.HeartbeatInterval = -1 },
		func(o *vivid.ActorSystemCoordinationOptions) { o.ElectionMin = time.Millisecond },
		func(o *vivid.ActorSystemCoordinationOptions) { o.ElectionMax = time.Second },
	} {
		o := valid
		change(&o)
		require.ErrorIs(t, o.Validate(), vivid.ErrorIllegalArgument)
	}
	option := vivid.WithActorSystemCoordination(valid)
	valid.Members[1] = "changed.example:9000"
	first := vivid.NewActorSystemOptions(option)
	require.Equal(t, "127.0.0.1:9000", first.CoordinationOptions.Members[1])
	require.Equal(t, 100*time.Millisecond, first.CoordinationOptions.HeartbeatInterval)
	first.CoordinationOptions.Members[1] = "other.example:9000"
	second := vivid.NewActorSystemOptions(option)
	require.Equal(t, "127.0.0.1:9000", second.CoordinationOptions.Members[1])
	third := vivid.NewActorSystemOptions(vivid.WithActorSystemOptions(second))
	second.CoordinationOptions.Members[1] = "later.example:9000"
	require.Equal(t, "127.0.0.1:9000", third.CoordinationOptions.Members[1])
}
