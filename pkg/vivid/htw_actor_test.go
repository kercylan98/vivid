package vivid_test

import (
	"testing"
	"time"

	"github.com/kercylan98/vivid/pkg/vivid"
)

func TestHTW_ScheduleOnce(t *testing.T) {
	sys := NewTestActorSystem(t)
	defer sys.Shutdown(true)

	expectedDelta := 100 * time.Millisecond

	sys.WaitAdd(1)
	ref := sys.ActorOf(func() vivid.Actor {
		return vivid.ActorFN(func(context vivid.ActorContext) {
			switch m := context.Message().(type) {
			case time.Time:
				delta := time.Since(m)
				if delta < expectedDelta {
					t.Errorf("delta is too small: %v", delta)
				} else if delta > expectedDelta*2 {
					t.Errorf("delta is too large: %v", delta)
				} else {
					t.Log("delta is correct", delta)
					sys.WaitDone()
				}
			}
		})
	}).Spawn()

	sys.ScheduleOnce("once", expectedDelta, ref, time.Now())

	sys.Wait()
}

func TestHTW_ScheduleInterval_AndCancel(t *testing.T) {
	sys := NewTestActorSystem(t)
	defer sys.Shutdown(true)

	sys.WaitAdd(1)
	ref := sys.ActorOf(func() vivid.Actor {
		return vivid.ActorFN(func(context vivid.ActorContext) {
			switch context.Message().(type) {
			case string:
				sys.WaitDone()
			case int:
				sys.WaitDone()
			}
		})
	}).Spawn()

	sys.ScheduleInterval("interval", 1*time.Millisecond, 5*time.Millisecond, ref, "ping")

	sys.Wait()

	sys.CancelSchedule("interval")

	sys.Wait()
}

func TestHTW_OverrideSameName(t *testing.T) {
	sys := NewTestActorSystem(t)
	defer sys.Shutdown(true)

	sys.WaitAdd(1)
	ref := sys.ActorOf(func() vivid.Actor {
		return vivid.ActorFN(func(context vivid.ActorContext) {
			switch context.Message().(type) {
			case string:
				sys.WaitDone()
			case int:
				sys.WaitDone()
			}
		})
	}).Spawn()

	sys.ScheduleOnce("dup", 100*time.Millisecond, ref, "old")
	sys.ScheduleOnce("dup", 1*time.Millisecond, ref, "new")

	sys.Wait()
}

func TestHTW_Cron_SkipWithoutRobfig(t *testing.T) {
	sys := NewTestActorSystem(t)
	defer sys.Shutdown(true)

	sys.WaitAdd(1)
	ref := sys.ActorOf(func() vivid.Actor {
		return vivid.ActorFN(func(context vivid.ActorContext) {
			switch context.Message().(type) {
			case string:
				sys.WaitDone()
			}
		})
	}).Spawn()
	sys.ScheduleCron("cron", "1/1 * * * * *", ref, "cron")

	sys.Wait()
}
