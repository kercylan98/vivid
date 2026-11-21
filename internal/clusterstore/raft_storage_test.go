package clusterstore

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func raftTestConfig() RaftConfig {
	return RaftConfig{GroupID: "orders", NodeID: 1, Voters: []uint64{3, 1, 2}}
}

func createRaftStorage(t testing.TB) (*RaftStorage, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "raft.wal")
	storage, err := CreateRaftStorage(path, raftTestConfig())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, storage.Close()) })
	return storage, path
}

func raftEntries(storage *RaftStorage) []RaftEntry {
	entries := make([]RaftEntry, len(storage.state.entries))
	for i, entry := range storage.state.entries {
		entries[i] = entry
		entries[i].Data = bytes.Clone(entry.Data)
	}
	return entries
}

func TestRaftStorage_Recovery(t *testing.T) {
	storage, path := createRaftStorage(t)
	initial := RaftUpdate{HardState: RaftHardState{Term: 2, Vote: 1, Commit: 1}, Entries: []RaftEntry{
		{Index: 1, Term: 1, Data: []byte("committed")},
		{Index: 2, Term: 2, Data: []byte("uncommitted")},
		{Index: 3, Term: 2, Data: []byte("old suffix")},
	}}
	require.NoError(t, storage.Save(initial))
	// 匹配的部分日志不能截断尚未包含在该批次中的后缀。
	sequence := storage.journal.Sequence()
	require.NoError(t, storage.Save(RaftUpdate{HardState: initial.HardState, Entries: initial.Entries[:2]}))
	require.Equal(t, sequence, storage.journal.Sequence(), "重复批次不增加刷盘")
	last, err := storage.LastIndex()
	require.NoError(t, err)
	require.EqualValues(t, 3, last)
	initial.Entries[0].Data[0] = 'x'
	entry, err := storage.Entry(1)
	require.NoError(t, err)
	require.Equal(t, []byte("committed"), entry.Data)
	entry.Data[0] = 'y'
	entry, err = storage.Entry(1)
	require.NoError(t, err)
	require.Equal(t, []byte("committed"), entry.Data)

	// 新任期可以清空旧投票；替换是追加的逻辑记录，不物理截断 WAL。
	require.NoError(t, storage.Save(RaftUpdate{HardState: RaftHardState{Term: 3, Commit: 1}}))
	beforeOffset := storage.journal.offset
	replacement := RaftUpdate{HardState: RaftHardState{Term: 3, Vote: 2, Commit: 2}, Entries: []RaftEntry{{Index: 2, Term: 3, Data: []byte("replacement")}}}
	require.NoError(t, storage.Save(replacement))
	require.Greater(t, storage.journal.offset, beforeOffset)
	_, err = storage.Entry(3)
	require.ErrorIs(t, err, ErrRaftEntryMissing)
	before, entries := storage.state.hard, raftEntries(storage)
	require.NoError(t, storage.Close())
	reopened, err := OpenRaftStorage(path, raftTestConfig())
	require.NoError(t, err)
	defer reopened.Close()
	hard, err := reopened.State()
	require.NoError(t, err)
	require.Equal(t, before, hard)
	require.Equal(t, entries, raftEntries(reopened))
	require.EqualValues(t, len("committedreplacement"), reopened.state.bytes)
	// 恢复后同任期仍不能改票。
	require.ErrorIs(t, reopened.Save(RaftUpdate{HardState: RaftHardState{Term: 3, Vote: 1, Commit: 2}}), ErrInvalidRaftUpdate)
	require.NoError(t, reopened.Save(RaftUpdate{HardState: hard, Entries: []RaftEntry{{Index: 3, Term: 3}, {Index: 4, Term: 3}}}))
	require.NoError(t, reopened.Close())
	check, err := OpenRaftStorage(path, raftTestConfig())
	require.NoError(t, err)
	require.EqualValues(t, 4, len(check.state.entries))
	require.Equal(t, before, check.state.hard, "追加未提交条目不能推进 Commit")
	require.NoError(t, check.Close())
}

func TestRaftStorage_Identity(t *testing.T) {
	config := raftTestConfig()
	path := filepath.Join(t.TempDir(), "identity.wal")
	storage, err := CreateRaftStorage(path, config)
	require.NoError(t, err)
	config.Voters[0] = 99
	copy := storage.Configuration()
	require.Equal(t, []uint64{1, 2, 3}, copy.Voters)
	copy.Voters[0] = 100
	require.Equal(t, []uint64{1, 2, 3}, storage.Configuration().Voters)
	require.NoError(t, storage.Close())
	for _, wrong := range []RaftConfig{
		{GroupID: "another", NodeID: 1, Voters: []uint64{1, 2, 3}},
		{GroupID: "orders", NodeID: 2, Voters: []uint64{1, 2, 3}},
		{GroupID: "orders", NodeID: 1, Voters: []uint64{1, 2, 4}},
	} {
		reopened, err := OpenRaftStorage(path, wrong)
		require.ErrorIs(t, err, ErrRaftIdentity)
		require.Nil(t, reopened)
	}
	correct := raftTestConfig()
	correct.Voters = []uint64{2, 3, 1}
	reopened, err := OpenRaftStorage(path, correct)
	require.NoError(t, err, "输入顺序不属于成员身份")
	require.NoError(t, reopened.Close())
	_, err = CreateRaftStorage(path, correct)
	require.Error(t, err, "创建不得覆盖已有身份")
}

func TestRaftStorage_InvalidConfig(t *testing.T) {
	for _, config := range []RaftConfig{
		{},
		{GroupID: string([]byte{255}), NodeID: 1, Voters: []uint64{1}},
		{GroupID: string(bytes.Repeat([]byte{'a'}, maxRaftGroupBytes+1)), NodeID: 1, Voters: []uint64{1}},
		{GroupID: "g", Voters: []uint64{1}},
		{GroupID: "g", NodeID: 1},
		{GroupID: "g", NodeID: 1, Voters: []uint64{1, 1}},
		{GroupID: "g", NodeID: 1, Voters: []uint64{0, 1}},
		{GroupID: "g", NodeID: 1, Voters: []uint64{2, 3}},
		{GroupID: "g", NodeID: 1, Voters: make([]uint64, maxRaftVoters+1)},
	} {
		path := filepath.Join(t.TempDir(), "invalid.wal")
		storage, err := CreateRaftStorage(path, config)
		require.ErrorIs(t, err, ErrInvalidRaftConfig)
		require.Nil(t, storage)
		_, err = os.Stat(path)
		require.ErrorIs(t, err, os.ErrNotExist)
		storage, err = OpenRaftStorage(path, config)
		require.ErrorIs(t, err, ErrInvalidRaftConfig)
		require.Nil(t, storage)
	}
}

func TestRaftStorage_InvalidUpdate(t *testing.T) {
	storage, _ := createRaftStorage(t)
	base := RaftUpdate{HardState: RaftHardState{Term: 3, Vote: 1, Commit: 1}, Entries: []RaftEntry{
		{Index: 1, Term: 1, Data: []byte("committed")}, {Index: 2, Term: 2}, {Index: 3, Term: 3, Data: []byte("old")},
	}}
	require.NoError(t, storage.Save(base))
	before, sequence, offset := raftEntries(storage), storage.journal.Sequence(), storage.journal.offset
	for _, hard := range []RaftHardState{
		{Term: 2, Vote: 1, Commit: 1}, {Term: 3, Commit: 1}, {Term: 3, Vote: 2, Commit: 1},
		{Term: 4, Vote: 99, Commit: 1}, {Term: 4, Commit: 0}, {Term: 4, Commit: 4},
	} {
		require.ErrorIs(t, storage.Save(RaftUpdate{HardState: hard}), ErrInvalidRaftUpdate)
	}
	for _, entries := range [][]RaftEntry{
		{{Index: 0, Term: 1}}, {{Index: 5, Term: 3}}, {{Index: ^uint64(0), Term: 3}},
		{{Index: 4, Term: 0}}, {{Index: 4, Term: 4}},
		{{Index: 4, Term: 3}, {Index: 6, Term: 3}},
		{{Index: 4, Term: 3}, {Index: 4, Term: 3}},
		{{Index: 4, Term: 2}}, {{Index: 2, Term: 2}, {Index: 3, Term: 1}},
		{{Index: 1, Term: 2}},                            // 不能覆盖已提交条目
		{{Index: 1, Term: 1, Data: []byte("different")}}, // 同任期同位置必须同负载
		{{Index: 2, Term: 3}, {Index: 3, Term: 3, Data: []byte("different")}},
		{{Index: 4, Term: 3, Data: make([]byte, maxRaftEntryBytes+1)}},
	} {
		require.ErrorIs(t, storage.Save(RaftUpdate{HardState: base.HardState, Entries: entries}), ErrInvalidRaftUpdate)
		require.Equal(t, before, raftEntries(storage))
		require.Equal(t, base.HardState, storage.state.hard)
		require.Equal(t, sequence, storage.journal.Sequence())
		require.Equal(t, offset, storage.journal.offset)
	}
	// 任期升高也不能用短后缀删除已提交位置，且不能提交到替换后不存在的位置。
	require.ErrorIs(t, storage.Save(RaftUpdate{HardState: RaftHardState{Term: 4, Commit: 3}, Entries: []RaftEntry{{Index: 2, Term: 4}}}), ErrInvalidRaftUpdate)
	var empty RaftStorage
	empty.config = raftTestConfig()
	_, err := empty.prepare(RaftUpdate{HardState: RaftHardState{Vote: 1}})
	require.ErrorIs(t, err, ErrInvalidRaftUpdate)
}

func TestRaftStorage_IOFailure(t *testing.T) {
	for _, mode := range []string{"short", "sync"} {
		t.Run(mode, func(t *testing.T) {
			storage, path := createRaftStorage(t)
			require.NoError(t, storage.Save(RaftUpdate{HardState: RaftHardState{Term: 1, Vote: 1}, Entries: []RaftEntry{{Index: 1, Term: 1}}}))
			require.NoError(t, storage.Save(RaftUpdate{HardState: RaftHardState{Term: 1, Vote: 1}, Entries: []RaftEntry{{Index: 2, Term: 1, Data: []byte("old")}, {Index: 3, Term: 1, Data: []byte("suffix")}}}))
			failure := errors.New("disk failure")
			fault := &failingJournalFile{journalFile: storage.journal.file}
			storage.journal.file = fault
			expected := failure
			if mode == "short" {
				fault.writeLimit, expected = journalRecordHeader+1, io.ErrShortWrite
			} else {
				fault.syncErr = failure
			}
			update := RaftUpdate{HardState: RaftHardState{Term: 2, Vote: 2, Commit: 2}, Entries: []RaftEntry{{Index: 2, Term: 2, Data: []byte("next")}}}
			require.ErrorIs(t, storage.Save(update), expected)
			require.Equal(t, RaftHardState{Term: 1, Vote: 1}, storage.state.hard, "刷盘失败不能发布新状态")
			require.Len(t, storage.state.entries, 3)
			require.Equal(t, []byte("old"), storage.state.entries[1].Data)
			require.Equal(t, []byte("suffix"), storage.state.entries[2].Data)
			_, err := storage.State()
			require.ErrorIs(t, err, ErrJournalFailed)
			_, err = storage.Entry(1)
			require.ErrorIs(t, err, ErrJournalFailed)
			_, err = storage.LastIndex()
			require.ErrorIs(t, err, ErrJournalFailed)
			require.ErrorIs(t, storage.Save(update), ErrJournalFailed)
			require.NoError(t, storage.Close())
			recovered, err := OpenRaftStorage(path, raftTestConfig())
			require.NoError(t, err)
			if mode == "sync" {
				require.Equal(t, update.HardState, recovered.state.hard)
				require.Len(t, recovered.state.entries, 2, "完整但未确认的批次允许在恢复后出现，必须整批生效")
			} else {
				require.Equal(t, RaftHardState{Term: 1, Vote: 1}, recovered.state.hard)
				require.Len(t, recovered.state.entries, 3)
			}
			require.NoError(t, recovered.Close())
		})
	}
}

func TestRaftStorage_InvalidReplay(t *testing.T) {
	config, err := normalizeRaftConfig(raftTestConfig())
	require.NoError(t, err)
	for _, payloads := range [][][]byte{
		nil,
		{[]byte("invalid")},
		{encodeRaftConfig(config), encodeRaftConfig(config)},
		{encodeRaftConfig(config), []byte("invalid")},
		{encodeRaftConfig(config), encodeRaftUpdate(RaftUpdate{HardState: RaftHardState{Term: 1, Vote: 99}})},
		{encodeRaftConfig(config), encodeRaftUpdate(RaftUpdate{HardState: RaftHardState{Term: 2}}), encodeRaftUpdate(RaftUpdate{HardState: RaftHardState{Term: 1}})},
	} {
		journal, path := createJournal(t)
		for _, payload := range payloads {
			_, err := journal.Append(payload)
			require.NoError(t, err)
		}
		require.NoError(t, journal.Close())
		before, err := os.ReadFile(path)
		require.NoError(t, err)
		storage, err := OpenRaftStorage(path, raftTestConfig())
		require.ErrorIs(t, err, ErrInvalidRaftState)
		require.Nil(t, storage)
		after, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, before, after)
		check, err := OpenJournal(path, nil)
		require.NoError(t, err, "恢复失败须释放锁")
		require.NoError(t, check.Close())
	}
}

func TestRaftStorage_InitializationTail(t *testing.T) {
	storage, path := createRaftStorage(t)
	require.NoError(t, storage.Close())
	image, err := os.ReadFile(path)
	require.NoError(t, err)
	for end := journalHeaderBytes; end < len(image); end++ {
		require.NoError(t, os.WriteFile(path, image[:end], 0600))
		recovered, err := OpenRaftStorage(path, raftTestConfig())
		require.ErrorIs(t, err, ErrInvalidRaftState, "未完成身份初始化不能作为新节点恢复")
		require.Nil(t, recovered)
		check, err := OpenJournal(path, nil)
		require.NoError(t, err)
		require.Zero(t, check.Sequence())
		require.NoError(t, check.Close())
	}
}

func TestRaftStorage_Closed(t *testing.T) {
	storage, _ := createRaftStorage(t)
	require.NoError(t, storage.Close())
	require.NoError(t, storage.Close())
	for _, target := range []*RaftStorage{storage, {}} {
		_, err := target.State()
		require.ErrorIs(t, err, ErrJournalClosed)
		_, err = target.LastIndex()
		require.ErrorIs(t, err, ErrJournalClosed)
		_, err = target.Entry(0)
		require.ErrorIs(t, err, ErrJournalClosed)
		require.Zero(t, target.RecoveredTailBytes())
		require.ErrorIs(t, target.Save(RaftUpdate{}), ErrJournalClosed)
		require.NoError(t, target.Close())
	}
}

func TestRaftStorage_Capacity(t *testing.T) {
	storage, _ := createRaftStorage(t)
	update := RaftUpdate{HardState: RaftHardState{Term: 1}, Entries: []RaftEntry{{Index: 1, Term: 1, Data: make([]byte, maxRaftEntryBytes)}, {Index: 2, Term: 1, Data: make([]byte, maxRaftEntryBytes)}}}
	require.ErrorIs(t, storage.Save(update), ErrRaftCapacity, "批次编码不得超出 WAL 上限")
	require.Zero(t, storage.state.hard.Term)
	offset := storage.journal.offset
	storage.journal.offset = maxJournalBytes
	require.ErrorIs(t, storage.Save(RaftUpdate{HardState: RaftHardState{Term: 1, Vote: 1}}), ErrJournalCapacity)
	hard, err := storage.State()
	require.NoError(t, err, "WAL 容量预检拒绝不等同于 I/O 失败")
	require.Zero(t, hard)
	storage.journal.offset = offset
	_, err = storage.prepare(RaftUpdate{Entries: make([]RaftEntry, maxRaftEntries+1)})
	require.ErrorIs(t, err, ErrRaftCapacity)
	// 直接构造合法的有界日志，隔离容量验证与磁盘刷盘次数。
	state := RaftStorage{config: raftTestConfig()}
	for i := range maxRaftEntries {
		state.state.entries = append(state.state.entries, RaftEntry{Index: uint64(i + 1), Term: 1})
	}
	state.state.hard.Term = 1
	_, err = state.prepare(RaftUpdate{HardState: state.state.hard, Entries: []RaftEntry{{Index: maxRaftEntries + 1, Term: 1}}})
	require.ErrorIs(t, err, ErrInvalidRaftUpdate)
	state.state.entries = state.state.entries[:32]
	value := make([]byte, maxRaftEntryBytes)
	for i := range state.state.entries {
		state.state.entries[i].Data = value
	}
	state.state.bytes = maxRaftLogBytes
	_, err = state.prepare(RaftUpdate{HardState: state.state.hard, Entries: []RaftEntry{{Index: 33, Term: 1, Data: []byte{1}}}})
	require.ErrorIs(t, err, ErrRaftCapacity)
	change, err := state.prepare(RaftUpdate{HardState: RaftHardState{Term: 2}, Entries: []RaftEntry{{Index: 2, Term: 2, Data: []byte("replacement")}}})
	require.NoError(t, err)
	state.state.install(change)
	require.Equal(t, maxRaftEntryBytes+len("replacement"), state.state.bytes)
}

func TestRaftStorage_AtomicTail(t *testing.T) {
	storage, path := createRaftStorage(t)
	base := RaftUpdate{HardState: RaftHardState{Term: 1, Vote: 1}, Entries: []RaftEntry{{Index: 1, Term: 1}}}
	require.NoError(t, storage.Save(base))
	prefix := storage.journal.offset
	update := RaftUpdate{HardState: RaftHardState{Term: 2, Vote: 2, Commit: 2}, Entries: []RaftEntry{{Index: 2, Term: 2, Data: []byte("atomic")}}}
	require.NoError(t, storage.Save(update))
	require.NoError(t, storage.Close())
	image, err := os.ReadFile(path)
	require.NoError(t, err)
	for end := int(prefix); end <= len(image); end++ {
		require.NoError(t, os.WriteFile(path, image[:end], 0600))
		recovered, err := OpenRaftStorage(path, raftTestConfig())
		require.NoError(t, err)
		if end == len(image) {
			require.Equal(t, update.HardState, recovered.state.hard)
			require.Len(t, recovered.state.entries, 2)
			require.Zero(t, recovered.RecoveredTailBytes())
		} else {
			require.Equal(t, base.HardState, recovered.state.hard)
			require.Len(t, recovered.state.entries, 1, "任期、投票、提交位置及新条目不得部分生效")
			require.Equal(t, int64(end)-prefix, recovered.RecoveredTailBytes())
		}
		require.NoError(t, recovered.Close())
	}
}

func TestRaftEncoding_Invalid(t *testing.T) {
	config, err := normalizeRaftConfig(raftTestConfig())
	require.NoError(t, err)
	metadata := encodeRaftConfig(config)
	require.True(t, validRaftConfigRecord(metadata))
	for size := range len(metadata) {
		require.False(t, validRaftConfigRecord(metadata[:size]))
	}
	require.False(t, validRaftConfigRecord(append(bytes.Clone(metadata), 0)))
	for _, offset := range []int{0, 8, 9, 19} {
		data := bytes.Clone(metadata)
		data[offset] = 255
		require.False(t, validRaftConfigRecord(data))
	}
	update := RaftUpdate{HardState: RaftHardState{Term: 2, Vote: 1, Commit: 1}, Entries: []RaftEntry{{Index: 1, Term: 1, Data: []byte("a")}, {Index: 2, Term: 2, Data: []byte("b")}}}
	data := encodeRaftUpdate(update)
	for size := range len(data) {
		_, err := decodeRaftUpdate(data[:size])
		require.ErrorIs(t, err, ErrInvalidRaftState)
	}
	for _, offset := range []int{0, 8, 33, raftUpdateHeaderBytes + 16} {
		invalid := bytes.Clone(data)
		if offset < 33 {
			invalid[offset] = 255
		} else {
			binary.LittleEndian.PutUint32(invalid[offset:], ^uint32(0))
		}
		_, err := decodeRaftUpdate(invalid)
		require.ErrorIs(t, err, ErrInvalidRaftState)
	}
	_, err = decodeRaftUpdate(append(bytes.Clone(data), 0))
	require.ErrorIs(t, err, ErrInvalidRaftState)
	decoded, err := decodeRaftUpdate(data)
	require.NoError(t, err)
	require.Equal(t, update, decoded)
}
