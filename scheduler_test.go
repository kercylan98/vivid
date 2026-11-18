package vivid_test

import (
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/stretchr/testify/require"
)

func TestScheduleOptionsDefaults(t *testing.T) {
	for _, options := range [][]vivid.ScheduleOption{
		nil, {nil}, {vivid.WithScheduleOptions(vivid.ScheduleOptions{})},
		{vivid.WithScheduleLocation(nil), vivid.WithSchedulerReference("")},
	} {
		opts := vivid.NewScheduleOptions(options...)
		require.Same(t, time.Local, opts.Location)
		require.NotEmpty(t, opts.Reference)
		require.NotEqual(t, opts.Reference, vivid.NewScheduleOptions(options...).Reference)
	}
	location := time.FixedZone("test", 8*60*60)
	opts := vivid.NewScheduleOptions(vivid.WithScheduleOptions(vivid.ScheduleOptions{Reference: "first", Location: location}), vivid.WithSchedulerReference("last"))
	require.Equal(t, "last", opts.Reference)
	require.Same(t, location, opts.Location)
}
