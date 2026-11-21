package clusterstore

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"unicode/utf8"
)

const (
	maxRaftVoters     = 256
	maxRaftGroupBytes = 128
	maxRaftEntryBytes = 2 << 20
	maxRaftEntries    = 1 << 16
	maxRaftLogBytes   = 64 << 20 // 当前保留日志的负载字节，不代表实际堆内存上限
)

var (
	ErrInvalidRaftConfig = errors.New("invalid fixed raft configuration")
	ErrInvalidRaftState  = errors.New("invalid persisted raft state")
	ErrInvalidRaftUpdate = errors.New("invalid raft state transition")
	ErrRaftIdentity      = errors.New("raft journal identity does not match configuration")
	ErrRaftCapacity      = errors.New("raft storage capacity exceeded")
	ErrRaftEntryMissing  = errors.New("raft entry is unavailable")
	ErrRaftCompacted     = errors.New("raft entry has been compacted")
)

// RaftConfig 固定投票成员与稳定身份，不保存通信地址，也不复用 Gossip 的启动实例 ID。
// NodeID 和 Voters 中的编号必须非零，GroupID 必须明确指定；本轮不支持成员变更。
type RaftConfig struct {
	GroupID string
	NodeID  uint64
	Voters  []uint64
}

// RaftHardState 的 Vote 为当前 Term 已投给的成员，0 表示本任期尚未投票。
// Commit 仅由共识层确认后传入，本地持久化无法判断是否已得到多数派授权。
type RaftHardState struct {
	Term   uint64
	Vote   uint64
	Commit uint64
}

// RaftEntry 保存共识日志中的位置、任期与不透明负载，空负载仍是有效条目。
type RaftEntry struct {
	Index uint64
	Term  uint64
	Data  []byte
}

// RaftUpdate 中的状态与条目共用一个持久化批次；HardState 必须提供完整值。
// Entries 按位置连续，仅在首个任期冲突处替换未提交后缀；匹配部分不截断原日志。
type RaftUpdate struct {
	HardState RaftHardState
	Entries   []RaftEntry
}

// RaftStorage 串行维护固定成员 Raft 的本地持久状态，不执行选举、复制或状态机应用。
// 必须通过 CreateRaftStorage 或 OpenRaftStorage 构造，使用后不可复制。
// 节点目录须独占，WAL 与锁文件不得被外部替换或通过路径别名并行打开。
type RaftStorage struct {
	journal  *Journal
	lock     *os.File
	path     string
	failed   bool
	snapshot [32]byte
	config   RaftConfig
	state    raftPersistentState
}

type raftPersistentState struct {
	baseIndex uint64
	baseTerm  uint64
	hard      RaftHardState
	entries   []RaftEntry
	bytes     int
}

type raftChange struct {
	hard     RaftHardState
	prefix   int
	bytes    int
	entries  []RaftEntry
	reserved []RaftEntry
}

// CreateRaftStorage 显式创建新节点，先持久化固定身份，不覆盖已有文件。
// 初始化失败可能留下文件，恢复时不会自动把空日志当作新节点。
func CreateRaftStorage(path string, config RaftConfig) (*RaftStorage, error) {
	config, err := normalizeRaftConfig(config)
	if err != nil {
		return nil, err
	}
	path, lock, err := lockRaftStorage(path)
	if err != nil {
		return nil, err
	}
	journal, err := CreateJournal(path)
	if err != nil {
		return nil, errors.Join(err, lock.Close())
	}
	if _, err = journal.Append(encodeRaftConfig(config)); err != nil {
		return nil, errors.Join(err, journal.Close(), lock.Close())
	}
	return &RaftStorage{journal: journal, lock: lock, path: path, config: config}, nil
}

// OpenRaftStorage 恢复时核对预期身份和固定成员；成员顺序不影响匹配。
// 任何批次无效都拒绝整个恢复结果，不把本地日志存在视为共识提交。
func OpenRaftStorage(path string, config RaftConfig) (*RaftStorage, error) {
	config, err := normalizeRaftConfig(config)
	if err != nil {
		return nil, err
	}
	path, lock, err := lockRaftStorage(path)
	if err != nil {
		return nil, err
	}
	storage := &RaftStorage{config: config, lock: lock, path: path}
	if err := storage.openJournal(); err != nil {
		return nil, errors.Join(err, lock.Close())
	}
	return storage, nil
}

func (storage *RaftStorage) openJournal() error {
	expected := encodeRaftConfig(storage.config)
	initialized := false
	journal, err := OpenJournal(storage.path, func(sequence uint64, payload []byte) error {
		if sequence == 1 {
			if !validRaftConfigRecord(payload) {
				return ErrInvalidRaftState
			}
			if !bytes.Equal(expected, payload) {
				return ErrRaftIdentity
			}
			initialized = true
			return nil
		}
		if isRaftCheckpoint(payload) {
			if sequence != 2 {
				return ErrInvalidRaftState
			}
			return storage.restoreCheckpoint(payload)
		}
		update, err := decodeRaftUpdate(payload)
		if err != nil {
			return err
		}
		change, err := storage.prepare(update)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidRaftState, err)
		}
		storage.state.install(change)
		return nil
	})
	if err != nil {
		return err
	}
	if !initialized {
		return errors.Join(ErrInvalidRaftState, journal.Close())
	}
	// 上次入口替换可能在目录同步前中断；回收旧文件前先固定本次恢复选择。
	if err := syncJournalDirectory(storage.path); err != nil {
		return errors.Join(err, journal.Close())
	}
	storage.journal = journal
	return nil
}

// Configuration 返回独立的成员列表，调用方不能修改持久化身份。
func (s *RaftStorage) Configuration() RaftConfig {
	config := s.config
	config.Voters = append([]uint64(nil), config.Voters...)
	return config
}

// State 只返回已完成本地刷盘的状态，I/O 失败后必须关闭并重新恢复。
func (s *RaftStorage) State() (RaftHardState, error) {
	if err := s.available(); err != nil {
		return RaftHardState{}, err
	}
	return s.state.hard, nil
}

func (s *RaftStorage) LastIndex() (uint64, error) {
	if err := s.available(); err != nil {
		return 0, err
	}
	return s.state.lastIndex(), nil
}

// LogBase 返回快照基准位置与任期，不读取快照负载。
func (s *RaftStorage) LogBase() (uint64, uint64, error) {
	if err := s.available(); err != nil {
		return 0, 0, err
	}
	return s.state.baseIndex, s.state.baseTerm, nil
}

// Term 可查询快照基准位置的任期，未压缩时位置 0 的任期为 0。
func (s *RaftStorage) Term(index uint64) (uint64, error) {
	if err := s.available(); err != nil {
		return 0, err
	}
	if index < s.state.baseIndex {
		return 0, ErrRaftCompacted
	}
	if index == s.state.baseIndex {
		return s.state.baseTerm, nil
	}
	if index > s.state.lastIndex() {
		return 0, ErrRaftEntryMissing
	}
	return s.state.entries[index-s.state.baseIndex-1].Term, nil
}

// RecoveredTailBytes 返回本次恢复丢弃的未完整写入批次字节数。
func (s *RaftStorage) RecoveredTailBytes() int64 {
	if s.journal == nil {
		return 0
	}
	return s.journal.RecoveredTailBytes()
}

// Entry 返回负载副本；未提交条目也可读取，由共识层决定其用途。
func (s *RaftStorage) Entry(index uint64) (RaftEntry, error) {
	if err := s.available(); err != nil {
		return RaftEntry{}, err
	}
	if s.state.baseIndex != 0 && index <= s.state.baseIndex {
		return RaftEntry{}, ErrRaftCompacted
	}
	if index == 0 || index > s.state.lastIndex() {
		return RaftEntry{}, ErrRaftEntryMissing
	}
	entry := s.state.entries[index-s.state.baseIndex-1]
	entry.Data = bytes.Clone(entry.Data)
	return entry, nil
}

// Save 校验并预留内存后刷盘，成功后才发布状态；调用期间输入不得修改。
// 写入错误的批次可能在恢复后存在，错误后不得依据旧状态继续参与投票或确认复制。
func (s *RaftStorage) Save(update RaftUpdate) error {
	if err := s.available(); err != nil {
		return err
	}
	change, err := s.prepare(update)
	if err != nil {
		return err
	}
	if len(change.entries) == 0 && change.prefix == len(s.state.entries) && change.hard == s.state.hard {
		return nil
	}
	if _, err = s.journal.Append(encodeRaftUpdate(update)); err != nil {
		return err
	}
	s.state.install(change)
	return nil
}

func (s *RaftStorage) Close() error {
	var err error
	if s.journal != nil {
		err = s.journal.Close()
	}
	if s.lock != nil {
		err = errors.Join(err, s.lock.Close())
		s.lock = nil
	}
	s.state = raftPersistentState{}
	return err
}

func (s *RaftStorage) available() error {
	if s.lock == nil {
		return ErrJournalClosed
	}
	if s.failed {
		return ErrJournalFailed
	}
	if s.journal == nil || s.journal.file == nil {
		return ErrJournalClosed
	}
	if s.journal.failed {
		return ErrJournalFailed
	}
	return nil
}

func lockRaftStorage(path string) (string, *os.File, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", nil, err
	}
	// 固定锁文件不参与 WAL 替换，也不删除；调用方不得通过别名创建第二个恢复入口。
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return "", nil, err
	}
	if err := lockJournal(lock); err != nil {
		return "", nil, errors.Join(err, lock.Close())
	}
	return path, lock, nil
}

func normalizeRaftConfig(config RaftConfig) (RaftConfig, error) {
	if config.GroupID == "" || len(config.GroupID) > maxRaftGroupBytes || !utf8.ValidString(config.GroupID) || config.NodeID == 0 || len(config.Voters) == 0 || len(config.Voters) > maxRaftVoters {
		return RaftConfig{}, ErrInvalidRaftConfig
	}
	config.Voters = append([]uint64(nil), config.Voters...)
	sort.Slice(config.Voters, func(i, j int) bool { return config.Voters[i] < config.Voters[j] })
	found := false
	for i, id := range config.Voters {
		if id == 0 || i > 0 && config.Voters[i-1] == id {
			return RaftConfig{}, ErrInvalidRaftConfig
		}
		found = found || id == config.NodeID
	}
	if !found {
		return RaftConfig{}, ErrInvalidRaftConfig
	}
	return config, nil
}

func (s *RaftStorage) prepare(update RaftUpdate) (raftChange, error) {
	state, hard := &s.state, update.HardState
	if len(update.Entries) > maxRaftEntries {
		return raftChange{}, ErrRaftCapacity
	}
	if hard.Term < state.hard.Term || hard.Commit < state.hard.Commit || hard.Commit < state.baseIndex || hard.Term < state.baseTerm || hard.Term == 0 && hard.Vote != 0 || hard.Term == state.hard.Term && state.hard.Vote != 0 && hard.Vote != state.hard.Vote {
		return raftChange{}, ErrInvalidRaftUpdate
	}
	if hard.Vote != 0 {
		member := sort.Search(len(s.config.Voters), func(i int) bool { return s.config.Voters[i] >= hard.Vote })
		if member == len(s.config.Voters) || s.config.Voters[member] != hard.Vote {
			return raftChange{}, ErrInvalidRaftUpdate
		}
	}
	change := raftChange{hard: hard, prefix: len(state.entries), bytes: state.bytes}
	encodedBytes := raftUpdateHeaderBytes
	var previous uint64
	diverged := false
	for i, entry := range update.Entries {
		if entry.Index <= state.baseIndex || entry.Index-state.baseIndex > maxRaftEntries || entry.Term == 0 || entry.Term > hard.Term || len(entry.Data) > maxRaftEntryBytes {
			return raftChange{}, ErrInvalidRaftUpdate
		}
		if i == 0 {
			if entry.Index-1 > state.lastIndex() {
				return raftChange{}, ErrInvalidRaftUpdate
			}
			previous = state.baseTerm
			if entry.Index-state.baseIndex > 1 {
				previous = state.entries[entry.Index-state.baseIndex-2].Term
			}
		} else if update.Entries[i-1].Index == ^uint64(0) || entry.Index != update.Entries[i-1].Index+1 {
			return raftChange{}, ErrInvalidRaftUpdate
		}
		if entry.Term < previous {
			return raftChange{}, ErrInvalidRaftUpdate
		}
		previous = entry.Term
		encodedBytes += raftEntryHeaderBytes + len(entry.Data)
		if encodedBytes > maxJournalRecordBytes {
			return raftChange{}, ErrRaftCapacity
		}
		position := int(entry.Index - state.baseIndex - 1)
		if position < len(state.entries) && entry.Term == state.entries[position].Term && !bytes.Equal(entry.Data, state.entries[position].Data) {
			return raftChange{}, ErrInvalidRaftUpdate
		}
		if !diverged {
			if position < len(state.entries) && entry.Term == state.entries[position].Term {
				continue
			}
			if entry.Index <= state.hard.Commit {
				return raftChange{}, ErrInvalidRaftUpdate
			}
			change.prefix = position
			diverged = true
			change.entries = make([]RaftEntry, 0, len(update.Entries)-i)
			for _, removed := range state.entries[position:] {
				change.bytes -= len(removed.Data)
			}
		}
		change.bytes += len(entry.Data)
		if change.bytes > maxRaftLogBytes {
			return raftChange{}, ErrRaftCapacity
		}
		change.entries = append(change.entries, entry)
	}
	if hard.Commit > state.baseIndex+uint64(change.prefix+len(change.entries)) {
		return raftChange{}, ErrInvalidRaftUpdate
	}
	for i := range change.entries {
		change.entries[i].Data = bytes.Clone(change.entries[i].Data)
	}
	// 在刷盘前准备扩容，未扩容时也只在刷盘成功后修改旧切片。
	change.reserved = state.entries[:change.prefix]
	needed := change.prefix + len(change.entries)
	if needed > cap(state.entries) {
		capacity := min(maxRaftEntries, max(needed, cap(state.entries)*2))
		change.reserved = make([]RaftEntry, change.prefix, capacity)
		copy(change.reserved, state.entries[:change.prefix])
	}
	return change, nil
}

func (s *raftPersistentState) lastIndex() uint64 {
	return s.baseIndex + uint64(len(s.entries))
}

func (s *raftPersistentState) install(change raftChange) {
	clear(s.entries[change.prefix:])
	s.entries = append(change.reserved, change.entries...)
	s.hard = change.hard
	s.bytes = change.bytes
}
