package gossip_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/gossip"
	"github.com/kercylan98/vivid/internal/gossip/endpoint"
	"github.com/kercylan98/vivid/pkg/bootstrap"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type restartMember struct {
	Address  string
	Status   endpoint.Status
	Revision uint64
}

type restartSnapshot struct {
	ID          string
	Fingerprint string
	Members     map[string]restartMember
}

type restartProcess struct {
	command *exec.Cmd
	input   io.WriteCloser
	output  *json.Decoder
	logs    bytes.Buffer
	stopped bool
}

func startRestartProcess(t *testing.T, address, seed string) *restartProcess {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)
	p := &restartProcess{command: exec.CommandContext(ctx, os.Args[0], "-test.run=^TestActor_GossipRestartProcess$")}
	p.command.Env = append(os.Environ(), "VIVID_GOSSIP_RESTART_ADDRESS="+address, "VIVID_GOSSIP_RESTART_SEED="+seed)
	p.command.Stderr = &p.logs
	var err error
	p.input, err = p.command.StdinPipe()
	require.NoError(t, err)
	output, err := p.command.StdoutPipe()
	require.NoError(t, err)
	p.output = json.NewDecoder(output)
	require.NoError(t, p.command.Start())
	t.Cleanup(func() {
		if !p.stopped {
			_ = p.command.Process.Kill()
			_ = p.command.Wait()
		}
		_ = p.input.Close()
		if t.Failed() {
			t.Logf("node %s: %s", address, p.logs.String())
		}
	})
	return p
}

func (p *restartProcess) snapshot(t *testing.T) restartSnapshot {
	t.Helper()
	require.NoError(t, json.NewEncoder(p.input).Encode("snapshot"))
	var snapshot restartSnapshot
	require.NoError(t, p.output.Decode(&snapshot))
	return snapshot
}

func (p *restartProcess) stop(t *testing.T, graceful bool) {
	t.Helper()
	if graceful {
		require.NoError(t, json.NewEncoder(p.input).Encode("stop"))
		require.NoError(t, p.command.Wait())
	} else {
		require.NoError(t, p.command.Process.Kill())
		require.Error(t, p.command.Wait())
	}
	p.stopped = true
}

// TestActor_GossipRestartProcess 作为独立节点运行，通过标准输入输出返回 Actor 线程内的快照。
func TestActor_GossipRestartProcess(t *testing.T) {
	address := os.Getenv("VIVID_GOSSIP_RESTART_ADDRESS")
	if address == "" {
		t.Skip("仅由重启回归启动子进程")
	}
	system := bootstrap.NewActorSystem(
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false))),
		vivid.WithActorSystemRemoting(address),
		vivid.WithActorSystemRemotingOption(
			vivid.WithActorSystemRemotingReconnect(2, 20*time.Millisecond, 100*time.Millisecond, 2, false),
			vivid.WithActorSystemRemotingReadTimeout(500*time.Millisecond),
			vivid.WithActorSystemRemotingHeartbeatInterval(100*time.Millisecond),
		),
	)
	require.NoError(t, system.Start())
	t.Cleanup(func() { require.NoError(t, system.Stop(3*time.Second)) })
	opts := vivid.NewActorSystemClusterOptions(
		vivid.WithActorSystemClusterLaunchDelay(0),
		vivid.WithActorSystemClusterGossipInterval(50*time.Millisecond),
		vivid.WithActorSystemClusterGracefulShutdownTimeout(time.Second),
	)
	opts.JoinTimeout = 100 * time.Millisecond
	if seed := os.Getenv("VIVID_GOSSIP_RESTART_SEED"); seed != "" {
		opts.Seeds = []string{seed}
	}
	ref, err := system.ActorOf(gossip.NewSnapshotActor(opts))
	require.NoError(t, err)
	input := json.NewDecoder(os.Stdin)
	output := json.NewEncoder(os.Stdout)
	for {
		var request string
		if input.Decode(&request) != nil || request == "stop" {
			return
		}
		result, err := system.Ask(ref, gossip.SnapshotRequest{}, time.Second).Result()
		require.NoError(t, err)
		snapshot := result.(gossip.SnapshotResponse)
		response := restartSnapshot{ID: snapshot.ID, Fingerprint: snapshot.View.Fingerprint(), Members: make(map[string]restartMember)}
		for _, member := range snapshot.View.Members().List() {
			response.Members[member.ID()] = restartMember{
				Address:  member.ActorRef.GetAddress(),
				Status:   member.Status,
				Revision: snapshot.View.Version().Get(member.ID()),
			}
		}
		require.NoError(t, output.Encode(response))
	}
}

func TestActor_GossipSameAddressRestart(t *testing.T) {
	for _, graceful := range []bool{true, false} {
		name := "crash"
		if graceful {
			name = "graceful"
		}
		t.Run(name, func(t *testing.T) {
			var addresses []string
			for range 2 {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)
				addresses = append(addresses, listener.Addr().String())
				require.NoError(t, listener.Close())
			}
			seed := startRestartProcess(t, addresses[0], "")
			oldID := seed.snapshot(t).ID
			peer := startRestartProcess(t, addresses[1], addresses[0])
			require.Eventually(t, func() bool {
				a, b := seed.snapshot(t), peer.snapshot(t)
				return len(a.Members) == 2 && a.Fingerprint == b.Fingerprint && a.Members[oldID].Status == endpoint.StatusUp
			}, 8*time.Second, 50*time.Millisecond)
			seed.stop(t, graceful)
			oldStatus := endpoint.StatusUp
			oldRevision := uint64(2)
			if graceful {
				oldStatus = endpoint.StatusRemoved
				oldRevision = 5
			}
			require.Eventually(t, func() bool {
				member := peer.snapshot(t).Members[oldID]
				return member.Status == oldStatus && member.Revision == oldRevision
			}, 5*time.Second, 50*time.Millisecond)
			// 新进程没有对端种子，只能由存活节点重新联络原种子地址来发现。
			current := startRestartProcess(t, addresses[0], "")
			newID := current.snapshot(t).ID
			require.NotEqual(t, oldID, newID)
			require.Eventually(t, func() bool {
				a, b := current.snapshot(t), peer.snapshot(t)
				return len(a.Members) == 3 && a.Fingerprint == b.Fingerprint &&
					a.Members[newID].Status == endpoint.StatusUp && a.Members[newID].Revision == 2 &&
					a.Members[oldID].Status == oldStatus && a.Members[oldID].Revision == oldRevision
			}, 8*time.Second, 50*time.Millisecond)
			for _, process := range []*restartProcess{current, peer} {
				snapshot := process.snapshot(t)
				assert.Equal(t, addresses[0], snapshot.Members[oldID].Address)
				assert.Equal(t, addresses[0], snapshot.Members[newID].Address)
			}
			t.Logf("same address recovered with independent revisions: old=%s/%d new=UP/2", oldStatus, oldRevision)
			current.stop(t, true)
			peer.stop(t, true)
		})
	}
}
