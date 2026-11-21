package actor

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/internal/clusterstore"
	"github.com/kercylan98/vivid/pkg/log"
	"github.com/kercylan98/vivid/pkg/ves"
	"github.com/stretchr/testify/require"
)

func coordinationTestOptions(path, address string) vivid.ActorSystemCoordinationOptions {
	return vivid.ActorSystemCoordinationOptions{GroupID: "managed", NodeID: 1, Members: map[uint64]string{1: address}, Path: path, Create: true,
		HeartbeatInterval: 5 * time.Millisecond, ElectionMin: 30 * time.Millisecond, ElectionMax: 60 * time.Millisecond}
}

// BenchmarkSystem_CoordinationStartStop 包含监听、WAL 首次创建和完整停止，不等待选主。
func BenchmarkSystem_CoordinationStartStop(b *testing.B) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(b, err)
	address, directory := listener.Addr().String(), b.TempDir()
	require.NoError(b, listener.Close())
	b.ReportAllocs()
	var index uint64
	for b.Loop() {
		index++
		options := coordinationTestOptions(filepath.Join(directory, fmt.Sprintf("node-%d.wal", index)), address)
		s := NewSystem(vivid.WithActorSystemCoordination(options), vivid.WithActorSystemRemoting(address),
			vivid.WithActorSystemLogger(&loggerProbe{}))
		if err := s.Start(); err != nil {
			b.Fatal(err)
		}
		if err := s.Stop(3 * time.Second); err != nil {
			b.Fatal(err)
		}
	}
}

func coordinationTestSystem(t testing.TB, options vivid.ActorSystemCoordinationOptions, extra ...vivid.ActorSystemOption) *System {
	t.Helper()
	s := NewSystem(append([]vivid.ActorSystemOption{vivid.WithActorSystemCoordination(options),
		vivid.WithActorSystemRemoting(options.Members[options.NodeID]),
		vivid.WithActorSystemLogger(log.NewTextLogger(log.WithLevel(log.LevelError), log.WithErrorStack(false)))}, extra...)...)
	t.Cleanup(func() {
		if !s.requestsStopped() {
			require.NoError(t, s.Stop(3*time.Second))
		}
	})
	return s
}

func waitCoordinationLeader(t testing.TB, s *System) vivid.CoordinationStatus {
	t.Helper()
	var status vivid.CoordinationStatus
	require.Eventually(t, func() bool {
		var err error
		status, err = s.CoordinationStatus(time.Second).Result()
		return err == nil && status.Role == vivid.CoordinationLeader
	}, 3*time.Second, 5*time.Millisecond)
	return status
}

func TestSystem_CoordinationCreateRecoverAndRollback(t *testing.T) {
	address := admissionSystemAddress(t)
	path := filepath.Join(t.TempDir(), "node.wal")
	o := coordinationTestOptions(path, address)
	s := coordinationTestSystem(t, o, vivid.WithActorSystemDefaultAskTimeout(time.Nanosecond))
	_, err := s.CoordinationStatus().Result()
	require.ErrorIs(t, err, vivid.ErrorActorSystemNotStarted)
	require.NoError(t, s.Start(), "本地存储初始化不使用业务默认 Ask 超时")
	status := waitCoordinationLeader(t, s)
	require.Equal(t, o.GroupID, status.GroupID)
	require.Equal(t, o.NodeID, status.NodeID)
	ref, err := s.FindActor(address + ":/@coordination")
	require.NoError(t, err)
	result, err := s.Ask(ref, clusterstore.RaftRuntimeExecute{Command: clusterstore.Command{Operation: clusterstore.Put, Key: "saved", Value: []byte("value")}}, time.Second).Result()
	require.NoError(t, err)
	executed := result.(clusterstore.RaftRuntimeExecution)
	require.NoError(t, executed.Err)
	require.True(t, executed.Result.Applied)
	require.NoError(t, s.Stop(3*time.Second))
	_, err = s.CoordinationStatus().Result()
	require.ErrorIs(t, err, vivid.ErrorActorSystemStopped)
	// 已存在的 WAL 不可再次创建；失败回滚必须释放监听与存储锁。
	duplicate := coordinationTestSystem(t, o)
	err = duplicate.Start()
	require.ErrorIs(t, err, vivid.ErrorActorSystemStartFailed)
	require.ErrorIs(t, err, vivid.ErrorCoordinationUnavailable)
	o.Create = false
	recovered := coordinationTestSystem(t, o)
	require.NoError(t, recovered.Start())
	status, err = recovered.CoordinationStatus(time.Second).Result()
	require.NoError(t, err)
	require.GreaterOrEqual(t, status.Applied, executed.Index)
	message, err := recovered.Ask(recovered.coordinationRef, clusterstore.RaftRuntimeLocalGet{Key: "saved"}, time.Second).Result()
	require.NoError(t, err)
	require.Equal(t, []byte("value"), message.(clusterstore.RaftRuntimeRecord).Value)
	require.NoError(t, recovered.Stop(3*time.Second))
	o.GroupID = "different"
	wrong := coordinationTestSystem(t, o)
	require.ErrorIs(t, wrong.Start(), clusterstore.ErrRaftIdentity)
	listener, err := net.Listen("tcp", address)
	require.NoError(t, err)
	require.NoError(t, listener.Close())
}

func TestSystem_CoordinationDoesNotWaitForMajority(t *testing.T) {
	address := admissionSystemAddress(t)
	o := coordinationTestOptions(filepath.Join(t.TempDir(), "node.wal"), address)
	o.Members[2], o.Members[3] = admissionSystemAddress(t), admissionSystemAddress(t)
	o.ElectionMin, o.ElectionMax = time.Hour, 2*time.Hour
	s := coordinationTestSystem(t, o)
	require.NoError(t, s.Start())
	status, err := s.CoordinationStatus(time.Second).Result()
	require.NoError(t, err)
	require.Equal(t, vivid.CoordinationFollower, status.Role)
	require.Zero(t, status.Leader)
	require.Zero(t, status.Commit)
}

func TestSystem_CoordinationRejectsInvalidAndMissingStorage(t *testing.T) {
	for _, mode := range []string{"missing", "no-remoting", "wrong-local", "timing"} {
		t.Run(mode, func(t *testing.T) {
			address := admissionSystemAddress(t)
			o := coordinationTestOptions(filepath.Join(t.TempDir(), "node.wal"), address)
			var extra []vivid.ActorSystemOption
			switch mode {
			case "missing":
				o.Create = false
			case "no-remoting":
				extra = append(extra, vivid.WithActorSystemRemoting(""))
			case "wrong-local":
				extra = append(extra, vivid.WithActorSystemRemoting(admissionSystemAddress(t)))
			case "timing":
				o.HeartbeatInterval = time.Second
			}
			s := coordinationTestSystem(t, o, extra...)
			err := s.Start()
			require.ErrorIs(t, err, vivid.ErrorActorSystemStartFailed)
			if mode == "missing" {
				require.ErrorIs(t, err, os.ErrNotExist)
			} else {
				require.ErrorIs(t, err, vivid.ErrorIllegalArgument)
			}
			_, err = os.Stat(o.Path)
			require.ErrorIs(t, err, os.ErrNotExist)
			listener, err := net.Listen("tcp", address)
			require.NoError(t, err)
			require.NoError(t, listener.Close())
		})
	}
	s := NewSystem(vivid.WithActorSystemLogger(&loggerProbe{}))
	require.NoError(t, s.Start())
	_, err := s.CoordinationStatus().Result()
	require.ErrorIs(t, err, vivid.ErrorCoordinationNotEnabled)
	require.NoError(t, s.Stop(time.Second))
}

func TestSystem_CoordinationSurvivesBusinessCleanup(t *testing.T) {
	o := coordinationTestOptions(filepath.Join(t.TempDir(), "node.wal"), admissionSystemAddress(t))
	s := coordinationTestSystem(t, o, vivid.WithActorSystemEnableMetrics(true),
		vivid.WithActorSystemClusterOptions(vivid.WithActorSystemClusterName("managed"), vivid.WithActorSystemClusterLaunchDelay(0), vivid.WithActorSystemClusterGossipInterval(5*time.Millisecond)))
	require.NoError(t, s.Start())
	waitCoordinationLeader(t, s)
	entered, release := make(chan struct{}, 1), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	checks := make(chan error, 1)
	_, err := s.ActorOf(vivid.ActorFN(func(ctx vivid.ActorContext) {
		if _, ok := ctx.Message().(*vivid.OnKill); ok {
			entered <- struct{}{}
			<-release
			_, err := ctx.System().CoordinationStatus(time.Second).Result()
			checks <- err
		}
	}))
	require.NoError(t, err)
	require.ErrorIs(t, s.Stop(10*time.Millisecond), vivid.ErrorActorSystemStopFailed)
	receiveRetryResult(t, entered)
	_, err = s.CoordinationStatus(time.Second).Result()
	require.NoError(t, err)
	listener, err := net.Listen("tcp", o.Members[1])
	if listener != nil {
		_ = listener.Close()
	}
	require.Error(t, err, "业务收尾中 Remoting 仍保留")
	once.Do(func() { close(release) })
	require.NoError(t, receiveRetryResult(t, checks))
	require.NoError(t, s.Stop(3*time.Second))
	listener, err = net.Listen("tcp", o.Members[1])
	require.NoError(t, err)
	require.NoError(t, listener.Close())
	storage, err := clusterstore.OpenRaftStorage(o.Path, clusterstore.RaftConfig{GroupID: o.GroupID, NodeID: 1, Voters: []uint64{1}})
	require.NoError(t, err, "停止完成后 WAL 锁已经释放")
	require.NoError(t, storage.Close())
}

func TestSystem_CoordinationPublicThreeNodePartitionRecovery(t *testing.T) {
	n := newRaftRuntimeNetwork(t, false, nil)
	// 托管系统各自登记清理，网络只负责最后关闭代理，避免重复停止同一实例。
	t.Cleanup(func() {
		for id, s := range n.systems {
			if !s.requestsStopped() {
				require.NoError(t, s.Stop(3*time.Second))
			}
			delete(n.systems, id)
		}
	})
	leader := n.leader(t, 1, 2, 3)
	n.command(t, leader, clusterstore.Command{Operation: clusterstore.Put, Key: "before", Value: []byte("saved")})
	members := make(map[uint64]string, 3)
	for id := uint64(1); id <= 3; id++ {
		require.NoError(t, n.systems[id].Stop(3*time.Second))
		members[id] = n.refs[id].GetAddress()
		n.refs[id], _ = NewRef(members[id], "/@coordination")
	}
	// 使用公开配置恢复同一持久化组，固定入口由系统托管，不手动 ActorOf。
	for id := uint64(1); id <= 3; id++ {
		o := vivid.ActorSystemCoordinationOptions{GroupID: "runtime", NodeID: id, Members: members, Path: n.paths[id],
			HeartbeatInterval: 20 * time.Millisecond, ElectionMin: 200 * time.Millisecond, ElectionMax: 400 * time.Millisecond}
		s := coordinationTestSystem(t, o, vivid.WithActorSystemRemoting(n.backends[id], members[id]),
			vivid.WithActorSystemRemotingOption(vivid.WithActorSystemRemotingReconnect(2, 20*time.Millisecond, 40*time.Millisecond, 2, false)))
		require.NoError(t, s.Start())
		n.systems[id] = s
	}
	leader = n.leader(t, 1, 2, 3)
	status, err := n.systems[leader].CoordinationStatus(time.Second).Result()
	require.NoError(t, err)
	require.Equal(t, vivid.CoordinationLeader, status.Role)
	require.Equal(t, []byte("saved"), n.get(t, leader, "before").Value)
	n.partition(leader)
	var majority []uint64
	for id := uint64(1); id <= 3; id++ {
		if id != leader {
			majority = append(majority, id)
		}
	}
	next := n.leader(t, majority...)
	index := n.command(t, next, clusterstore.Command{Operation: clusterstore.Put, Key: "after", Value: []byte("partition")})
	n.partition()
	require.Eventually(t, func() bool {
		for id := uint64(1); id <= 3; id++ {
			status, err := n.systems[id].CoordinationStatus(time.Second).Result()
			if err != nil || status.Applied < index {
				return false
			}
		}
		return true
	}, 5*time.Second, 20*time.Millisecond)
	for id := uint64(1); id <= 3; id++ {
		require.Equal(t, []byte("partition"), n.get(t, id, "after").Value)
	}
}

func TestSystem_CoordinationStopDuringStartAndLaterRollback(t *testing.T) {
	for _, interrupted := range []bool{false, true} {
		t.Run(map[bool]string{false: "later-failure", true: "stop-during-start"}[interrupted], func(t *testing.T) {
			o := coordinationTestOptions(filepath.Join(t.TempDir(), "node.wal"), admissionSystemAddress(t))
			s := coordinationTestSystem(t, o)
			entered, release := make(chan struct{}, 1), make(chan struct{})
			var once sync.Once
			defer once.Do(func() { close(release) })
			if interrupted {
				s.eventStream = &startupEventStream{EventStream: s.eventStream, observe: func(event vivid.Message) {
					if spawned, ok := event.(ves.ActorSpawnedEvent); ok && spawned.ActorRef.GetPath() == "/@coordination" {
						entered <- struct{}{}
						<-release
					}
				}}
			} else {
				s.virtualCoordinator.options.ActivationConcurrency = -1
			}
			result := make(chan error, 1)
			go func() { result <- s.Start() }()
			if interrupted {
				receiveRetryResult(t, entered)
				require.ErrorIs(t, s.Stop(10*time.Millisecond), vivid.ErrorActorSystemStopFailed)
				once.Do(func() { close(release) })
				require.ErrorIs(t, receiveRetryResult(t, result), vivid.ErrorActorSystemStopped)
				if !s.requestsStopped() {
					require.NoError(t, s.Stop(3*time.Second))
				}
			} else {
				require.ErrorIs(t, receiveRetryResult(t, result), vivid.ErrorActorSystemStartFailed)
			}
			storage, err := clusterstore.OpenRaftStorage(o.Path, clusterstore.RaftConfig{GroupID: o.GroupID, NodeID: 1, Voters: []uint64{1}})
			require.NoError(t, err)
			require.NoError(t, storage.Close())
			listener, err := net.Listen("tcp", o.Members[1])
			require.NoError(t, err)
			require.NoError(t, listener.Close())
		})
	}
}
