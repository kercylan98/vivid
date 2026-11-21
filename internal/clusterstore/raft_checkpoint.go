package clusterstore

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	raftCheckpointRecord = 3
	raftCheckpointBytes  = 8 + 1 + 8*5 + sha256.Size
)

// RaftSnapshot 保存已应用状态和相应日志任期，不携带多数派授权。
type RaftSnapshot struct {
	Index uint64
	Term  uint64
	Data  []byte
}

// Snapshot 返回最近成功安装的本地快照，尚无快照时返回零值。
// 数据独立于磁盘及后续调用，调用方恢复它后仍需应用到 Commit 的保留日志。
func (s *RaftStorage) Snapshot() (RaftSnapshot, error) {
	if err := s.available(); err != nil {
		return RaftSnapshot{}, err
	}
	if s.state.baseIndex == 0 {
		return RaftSnapshot{}, nil
	}
	data, err := s.readSnapshot(s.snapshot, s.state.baseIndex)
	if err != nil {
		return RaftSnapshot{}, err
	}
	return RaftSnapshot{Index: s.state.baseIndex, Term: s.state.baseTerm, Data: data}, nil
}

// Compact 安装本地已应用的 Store 快照并回收此前的日志批次。
// 快照位置必须向前且不超过 Commit，内容须由调用方从对应位置的状态机生成。
// 不用于接收远端快照，不替代共识资格检查；调用期间输入不可修改。
// 切换入口的 I/O 错误要求关闭并恢复。安装后清理残留失败会返回错误，但新状态仍可用。
func (s *RaftStorage) Compact(data []byte) error {
	return s.compact(data, nil)
}

// compact 的阶段通知仅用于固定崩溃窗口测试，不进入普通写入路径。
func (s *RaftStorage) compact(data []byte, stage func(string) error) error {
	if err := s.available(); err != nil {
		return err
	}
	store, err := Restore(data)
	if err != nil {
		return err
	}
	index := store.Index()
	if index <= s.state.baseIndex || index > s.state.hard.Commit {
		return ErrInvalidRaftUpdate
	}
	term, err := s.Term(index)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(data)
	if err := s.writeSnapshot(data, digest); err != nil {
		return err
	}
	notify := func(name string) error {
		if stage != nil {
			return stage(name)
		}
		return nil
	}
	if err := notify("snapshot"); err != nil {
		return err
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	candidatePath := s.path + ".rewrite-" + hex.EncodeToString(nonce[:])
	candidate, err := CreateJournal(candidatePath)
	if err != nil {
		return err
	}
	defer func() {
		candidate.Close()
		os.Remove(candidatePath)
	}()
	if _, err := candidate.Append(encodeRaftConfig(s.config)); err != nil {
		return err
	}
	hard := s.state.hard
	hard.Commit = index
	if _, err := candidate.Append(encodeRaftCheckpoint(index, term, hard, digest)); err != nil {
		return err
	}
	start := int(index - s.state.baseIndex)
	for start < len(s.state.entries) {
		end, size := start, raftUpdateHeaderBytes
		for end < len(s.state.entries) && size+raftEntryHeaderBytes+len(s.state.entries[end].Data) <= maxJournalRecordBytes {
			size += raftEntryHeaderBytes + len(s.state.entries[end].Data)
			end++
		}
		hard.Commit = min(s.state.hard.Commit, s.state.entries[end-1].Index)
		if _, err := candidate.Append(encodeRaftUpdate(RaftUpdate{HardState: hard, Entries: s.state.entries[start:end]})); err != nil {
			return err
		}
		start = end
	}
	if err := candidate.Close(); err != nil {
		return err
	}
	if err := notify("candidate"); err != nil {
		return err
	}
	// 固定锁文件跨 WAL 关闭与替换持有，其他框架实例不能在中间接管。
	s.failed = true
	if err := s.journal.Close(); err != nil {
		return err
	}
	if err := notify("closed"); err != nil {
		return err
	}
	if err := replaceJournal(candidatePath, s.path); err != nil {
		return err
	}
	if err := notify("published"); err != nil {
		return err
	}
	recovered := &RaftStorage{config: s.config, path: s.path}
	if err := recovered.openJournal(); err != nil {
		return err
	}
	s.journal, s.state, s.snapshot = recovered.journal, recovered.state, recovered.snapshot
	s.failed = false
	if err := notify("installed"); err != nil {
		return err
	}
	return s.Reclaim()
}

func isRaftCheckpoint(data []byte) bool {
	return len(data) >= 9 && string(data[:8]) == raftRecordMagic && data[8] == raftCheckpointRecord
}

func encodeRaftCheckpoint(index, term uint64, hard RaftHardState, digest [sha256.Size]byte) []byte {
	data := append([]byte(raftRecordMagic), raftCheckpointRecord)
	for _, value := range []uint64{index, term, hard.Term, hard.Vote, hard.Commit} {
		data = binary.LittleEndian.AppendUint64(data, value)
	}
	return append(data, digest[:]...)
}

func (s *RaftStorage) restoreCheckpoint(data []byte) error {
	if len(data) != raftCheckpointBytes || !isRaftCheckpoint(data) {
		return ErrInvalidRaftState
	}
	index, term := binary.LittleEndian.Uint64(data[9:]), binary.LittleEndian.Uint64(data[17:])
	hard := RaftHardState{Term: binary.LittleEndian.Uint64(data[25:]), Vote: binary.LittleEndian.Uint64(data[33:]), Commit: binary.LittleEndian.Uint64(data[41:])}
	if index == 0 || term == 0 || hard.Term < term || hard.Commit != index {
		return ErrInvalidRaftState
	}
	copy(s.snapshot[:], data[49:])
	if _, err := s.readSnapshot(s.snapshot, index); err != nil {
		return err
	}
	s.state = raftPersistentState{baseIndex: index, baseTerm: term}
	change, err := s.prepare(RaftUpdate{HardState: hard})
	if err != nil {
		return errors.Join(ErrInvalidRaftState, err)
	}
	s.state.install(change)
	return nil
}

func (s *RaftStorage) snapshotPath(digest [sha256.Size]byte) string {
	return s.path + ".snapshot-" + hex.EncodeToString(digest[:])
}

func (s *RaftStorage) readSnapshot(digest [sha256.Size]byte, index uint64) ([]byte, error) {
	data, err := readRaftSnapshotFile(s.snapshotPath(digest))
	if err != nil {
		return nil, err
	}
	if len(data) > maxSnapshotBytes || sha256.Sum256(data) != digest {
		return nil, ErrInvalidSnapshot
	}
	store, err := Restore(data)
	if err != nil {
		return nil, err
	}
	if store.Index() != index {
		return nil, ErrInvalidSnapshot
	}
	return data, nil
}

func (s *RaftStorage) writeSnapshot(data []byte, digest [sha256.Size]byte) error {
	path := s.snapshotPath(digest)
	if existing, err := readRaftSnapshotFile(path); err == nil {
		if !bytes.Equal(existing, data) {
			return ErrInvalidSnapshot
		}
		return syncJournalDirectory(path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	temporary := s.path + ".staging-" + hex.EncodeToString(nonce[:])
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer os.Remove(temporary)
	err = writeJournalAt(file, data, 0)
	if err == nil {
		err = file.Sync()
	}
	err = errors.Join(err, file.Close())
	if err != nil {
		return err
	}
	return publishJournal(temporary, path)
}

func readRaftSnapshotFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		return nil, errors.Join(err, file.Close())
	}
	if !info.Mode().IsRegular() || info.Size() > maxSnapshotBytes || info.Size() < 0 {
		return nil, errors.Join(ErrInvalidSnapshot, file.Close())
	}
	// 已有文件受节点独占目录约束，按有界文件大小一次分配，不按读取增长反复复制。
	data := make([]byte, int(info.Size()))
	_, err = io.ReadFull(file, data)
	return data, errors.Join(err, file.Close())
}

// Reclaim 只删除本恢复入口的旧快照及重写文件，不改动当前 WAL、锁和当前快照。
// 完成入口切换后才能调用；删除失败可以重试，不导致持久状态回退。
// 调用方须为节点提供独占目录，不得通过符号链接或硬链接建立第二个 WAL 入口。
func (s *RaftStorage) Reclaim() error {
	if err := s.available(); err != nil {
		return err
	}
	entries, err := os.ReadDir(filepath.Dir(s.path))
	if err != nil {
		return err
	}
	base := filepath.Base(s.path)
	current := filepath.Base(s.snapshotPath(s.snapshot))
	for _, entry := range entries {
		name := entry.Name()
		if name == current || entry.IsDir() {
			continue
		}
		remove := false
		for _, item := range []struct {
			prefix string
			length int
		}{{base + ".snapshot-", 64}, {base + ".rewrite-", 32}, {base + ".staging-", 32}} {
			if strings.HasPrefix(name, item.prefix) && len(name) == len(item.prefix)+item.length {
				_, err := hex.DecodeString(name[len(item.prefix):])
				remove = err == nil
			}
		}
		if remove {
			if err := os.Remove(filepath.Join(filepath.Dir(s.path), name)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	return syncJournalDirectory(s.path)
}
