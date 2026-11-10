package mailbox

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testEnvelopHandler func(envelop vivid.Envelop)

func (fn testEnvelopHandler) HandleEnvelop(envelop vivid.Envelop) {
	fn(envelop)
}

func receiveEnvelop(t *testing.T, received <-chan vivid.Envelop) vivid.Envelop {
	t.Helper()
	select {
	case envelop := <-received:
		return envelop
	case <-time.After(3 * time.Second):
		t.Fatal("receive envelop timeout")
		return nil
	}
}

func TestUnboundedMailbox_Enqueue(t *testing.T) {
	const count = 4096
	received := make(chan vivid.Envelop, count)
	m := NewUnboundedMailbox(4, testEnvelopHandler(func(envelop vivid.Envelop) {
		received <- envelop
	}))

	for i := 0; i < count; i++ {
		m.Enqueue(NewEnvelop(false, nil, nil, i))
	}
	for i := 0; i < count; i++ {
		assert.Equal(t, i, receiveEnvelop(t, received).Message())
	}
}

func TestUnboundedMailbox_ConcurrentEnqueue(t *testing.T) {
	t.Run("envelop", func(t *testing.T) {
		testConcurrentEnqueue(t, func(m *UnboundedMailbox, message vivid.Message) {
			m.Enqueue(NewEnvelop(false, nil, nil, message))
		})
	})
	t.Run("message", func(t *testing.T) {
		testConcurrentEnqueue(t, func(m *UnboundedMailbox, message vivid.Message) {
			EnqueueMessage[vivid.ActorRef](m, nil, nil, message)
		})
	})
}

func testConcurrentEnqueue(t *testing.T, enqueue func(m *UnboundedMailbox, message vivid.Message)) {
	t.Helper()
	type message struct {
		producer int
		index    int
	}
	const producers, count = 8, 1024
	var active, concurrent int32
	received := make(chan vivid.Envelop, producers*count)
	m := NewUnboundedMailbox(4, testEnvelopHandler(func(envelop vivid.Envelop) {
		if atomic.AddInt32(&active, 1) != 1 {
			atomic.StoreInt32(&concurrent, 1)
		}
		runtime.Gosched()
		received <- envelop
		atomic.AddInt32(&active, -1)
	}))

	var wg sync.WaitGroup
	for producer := 0; producer < producers; producer++ {
		wg.Add(1)
		go func(producer int) {
			defer wg.Done()
			for i := 0; i < count; i++ {
				enqueue(m, message{producer: producer, index: i})
			}
		}(producer)
	}
	wg.Wait()

	var counts [producers]int
	for i := 0; i < producers*count; i++ {
		msg := receiveEnvelop(t, received).Message().(message)
		require.Equal(t, counts[msg.producer], msg.index)
		counts[msg.producer]++
	}
	for _, total := range counts {
		assert.Equal(t, count, total)
	}
	assert.Zero(t, atomic.LoadInt32(&concurrent))
}

func TestUnboundedMailbox_Pause(t *testing.T) {
	received := make(chan vivid.Envelop, 4)
	m := NewUnboundedMailbox(4, testEnvelopHandler(func(envelop vivid.Envelop) {
		received <- envelop
	}))
	m.Pause()
	assert.True(t, m.IsPaused())
	user := NewEnvelop(false, nil, nil, "user")
	m.Enqueue(user)

	m.lock.Lock()
	status := m.status
	m.lock.Unlock()
	assert.EqualValues(t, idle, status)

	system := NewEnvelop(true, nil, nil, "system")
	m.Enqueue(system)
	assert.Same(t, system, receiveEnvelop(t, received))
	select {
	case envelop := <-received:
		t.Fatalf("received user envelop while paused: %v", envelop.Message())
	default:
	}

	m.Resume()
	assert.False(t, m.IsPaused())
	assert.Same(t, user, receiveEnvelop(t, received))
}

func TestUnboundedMailbox_PauseDuringBatch(t *testing.T) {
	const count = 1024
	received := make(chan vivid.Envelop, count+1)
	var m *UnboundedMailbox
	m = NewUnboundedMailbox(4, testEnvelopHandler(func(envelop vivid.Envelop) {
		if envelop.Message() == 0 {
			m.Pause()
		}
		received <- envelop
	}))
	m.Pause()
	for i := 0; i < count; i++ {
		m.Enqueue(NewEnvelop(false, nil, nil, i))
	}
	m.Resume()
	assert.Equal(t, 0, receiveEnvelop(t, received).Message())
	m.Enqueue(NewEnvelop(false, nil, nil, count))
	m.Enqueue(NewEnvelop(true, nil, nil, "system"))
	assert.Equal(t, "system", receiveEnvelop(t, received).Message())
	m.Resume()
	for i := 1; i <= count; i++ {
		assert.Equal(t, i, receiveEnvelop(t, received).Message())
	}
}

func TestUnboundedMailbox_SystemPriority(t *testing.T) {
	received := make(chan vivid.Envelop, 4)
	release := make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	m := NewUnboundedMailbox(4, testEnvelopHandler(func(envelop vivid.Envelop) {
		received <- envelop
		if envelop.Message() == 0 {
			<-release
		}
	}))
	m.Pause()
	m.Enqueue(NewEnvelop(false, nil, nil, 0))
	m.Enqueue(NewEnvelop(false, nil, nil, 1))
	m.Resume()
	assert.Equal(t, 0, receiveEnvelop(t, received).Message())
	m.Enqueue(NewEnvelop(false, nil, nil, 2))
	m.Enqueue(NewEnvelop(true, nil, nil, "system"))
	once.Do(func() { close(release) })
	assert.Equal(t, "system", receiveEnvelop(t, received).Message())
	assert.Equal(t, 1, receiveEnvelop(t, received).Message())
	assert.Equal(t, 2, receiveEnvelop(t, received).Message())
}

func TestUnboundedMailbox_Resume(t *testing.T) {
	const count = 512
	received := make(chan vivid.Envelop, count)
	m := NewUnboundedMailbox(4, testEnvelopHandler(func(envelop vivid.Envelop) {
		received <- envelop
	}))
	for i := 0; i < count; i++ {
		m.Pause()
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			m.Enqueue(NewEnvelop(false, nil, nil, i))
		}()
		go func() {
			defer wg.Done()
			m.Resume()
			m.Resume()
		}()
		wg.Wait()
		assert.Equal(t, i, receiveEnvelop(t, received).Message())
	}
}
