package clusterstore

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func FuzzRaftUpdate_Decode(f *testing.F) {
	f.Add([]byte{})
	f.Add(encodeRaftUpdate(RaftUpdate{}))
	f.Add(encodeRaftUpdate(RaftUpdate{HardState: RaftHardState{Term: 1, Vote: 1, Commit: 1}, Entries: []RaftEntry{{Index: 1, Term: 1, Data: []byte("command")}}}))
	config, err := normalizeRaftConfig(raftTestConfig())
	require.NoError(f, err)
	f.Add(encodeRaftConfig(config))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > maxJournalRecordBytes+1 {
			return
		}
		if validRaftConfigRecord(data) {
			require.GreaterOrEqual(t, len(data), raftConfigHeaderBytes)
		}
		update, err := decodeRaftUpdate(data)
		if err != nil {
			require.ErrorIs(t, err, ErrInvalidRaftState)
			return
		}
		require.Equal(t, data, encodeRaftUpdate(update))
		storage := &RaftStorage{config: config}
		change, err := storage.prepare(update)
		if err != nil {
			require.Empty(t, storage.state.entries)
			require.Zero(t, storage.state.hard)
			return
		}
		storage.state.install(change)
		before := raftEntries(storage)
		clear(data)
		require.Equal(t, before, raftEntries(storage), "安装状态不借用解码输入")
	})
}

func FuzzRaftState_Transitions(f *testing.F) {
	f.Add([]byte{0, 1, 1, 2, 3, 4, 5, 0, 1})
	f.Add([]byte{1, 1, 1, 2, 2, 4, 4, 6, 7})
	f.Fuzz(func(t *testing.T, operations []byte) {
		if len(operations) > 128 {
			return
		}
		config, err := normalizeRaftConfig(raftTestConfig())
		require.NoError(t, err)
		storage, recovered := &RaftStorage{config: config}, &RaftStorage{config: config}
		for _, operation := range operations {
			before, entries := storage.state.hard, raftEntries(storage)
			update := RaftUpdate{HardState: before}
			switch operation % 8 {
			case 0:
				update.HardState.Term++
				update.HardState.Vote = 0
			case 1:
				update.HardState.Term = max(1, before.Term)
				update.Entries = []RaftEntry{{Index: uint64(len(entries)) + 1, Term: update.HardState.Term, Data: []byte{operation}}}
			case 2:
				update.HardState.Commit = uint64(len(entries))
			case 3:
				update.HardState.Vote = uint64(operation%3 + 1)
			case 4:
				update.HardState.Term++
				update.HardState.Vote = 0
				update.Entries = []RaftEntry{{Index: before.Commit + 1, Term: update.HardState.Term, Data: []byte{operation}}}
			case 5:
				if len(entries) != 0 {
					update.Entries = entries[:int(operation)%len(entries)+1]
				}
			case 6:
				update.HardState.Commit = uint64(len(entries)) + 1
			case 7:
				if len(entries) != 0 {
					entry := entries[0]
					entry.Data = append(bytes.Clone(entry.Data), operation)
					update.Entries = []RaftEntry{entry}
				}
			}
			change, err := storage.prepare(update)
			if err != nil {
				require.Equal(t, before, storage.state.hard)
				require.Equal(t, entries, raftEntries(storage))
				continue
			}
			storage.state.install(change)
			require.GreaterOrEqual(t, storage.state.hard.Term, before.Term)
			require.GreaterOrEqual(t, storage.state.hard.Commit, before.Commit)
			if storage.state.hard.Term == before.Term && before.Vote != 0 {
				require.Equal(t, before.Vote, storage.state.hard.Vote)
			}
			require.GreaterOrEqual(t, uint64(len(storage.state.entries)), storage.state.hard.Commit)
			for index := uint64(0); index < before.Commit; index++ {
				require.Equal(t, entries[index], storage.state.entries[index], "已提交前缀不可变化")
			}
			var logicalBytes int
			var previous uint64
			for i, entry := range storage.state.entries {
				require.EqualValues(t, i+1, entry.Index)
				require.GreaterOrEqual(t, entry.Term, previous)
				require.LessOrEqual(t, entry.Term, storage.state.hard.Term)
				previous = entry.Term
				logicalBytes += len(entry.Data)
			}
			require.Equal(t, logicalBytes, storage.state.bytes)
			// 以持久化编码重放相同更新，核对恢复后的确定性和完整前缀。
			decoded, err := decodeRaftUpdate(encodeRaftUpdate(update))
			require.NoError(t, err)
			replayed, err := recovered.prepare(decoded)
			require.NoError(t, err)
			recovered.state.install(replayed)
			require.Equal(t, storage.state.hard, recovered.state.hard)
			require.Equal(t, raftEntries(storage), raftEntries(recovered))
		}
	})
}
