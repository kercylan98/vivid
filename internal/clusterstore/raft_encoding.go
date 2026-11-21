package clusterstore

import (
	"bytes"
	"encoding/binary"
)

const (
	raftRecordMagic       = "VIVIDRF1"
	raftConfigRecord      = 1
	raftUpdateRecord      = 2
	raftConfigHeaderBytes = 21
	raftUpdateHeaderBytes = 37
	raftEntryHeaderBytes  = 20
)

func encodeRaftConfig(config RaftConfig) []byte {
	data := make([]byte, 0, raftConfigHeaderBytes+len(config.GroupID)+len(config.Voters)*8)
	data = append(data, raftRecordMagic...)
	data = append(data, raftConfigRecord)
	data = binary.LittleEndian.AppendUint16(data, uint16(len(config.GroupID)))
	data = binary.LittleEndian.AppendUint64(data, config.NodeID)
	data = binary.LittleEndian.AppendUint16(data, uint16(len(config.Voters)))
	data = append(data, config.GroupID...)
	for _, id := range config.Voters {
		data = binary.LittleEndian.AppendUint64(data, id)
	}
	return data
}

func validRaftConfigRecord(data []byte) bool {
	if len(data) < raftConfigHeaderBytes || string(data[:8]) != raftRecordMagic || data[8] != raftConfigRecord {
		return false
	}
	groupBytes := int(binary.LittleEndian.Uint16(data[9:]))
	count := int(binary.LittleEndian.Uint16(data[19:]))
	if groupBytes > maxRaftGroupBytes || count > maxRaftVoters || len(data) != raftConfigHeaderBytes+groupBytes+count*8 {
		return false
	}
	config := RaftConfig{GroupID: string(data[raftConfigHeaderBytes : raftConfigHeaderBytes+groupBytes]), NodeID: binary.LittleEndian.Uint64(data[11:]), Voters: make([]uint64, count)}
	for i := range config.Voters {
		config.Voters[i] = binary.LittleEndian.Uint64(data[raftConfigHeaderBytes+groupBytes+i*8:])
	}
	canonical, err := normalizeRaftConfig(config)
	return err == nil && bytes.Equal(data, encodeRaftConfig(canonical))
}

func encodeRaftUpdate(update RaftUpdate) []byte {
	size := raftUpdateHeaderBytes + len(update.Entries)*raftEntryHeaderBytes
	for _, entry := range update.Entries {
		size += len(entry.Data)
	}
	data := make([]byte, 0, size)
	data = append(data, raftRecordMagic...)
	data = append(data, raftUpdateRecord)
	data = binary.LittleEndian.AppendUint64(data, update.HardState.Term)
	data = binary.LittleEndian.AppendUint64(data, update.HardState.Vote)
	data = binary.LittleEndian.AppendUint64(data, update.HardState.Commit)
	data = binary.LittleEndian.AppendUint32(data, uint32(len(update.Entries)))
	for _, entry := range update.Entries {
		data = binary.LittleEndian.AppendUint64(data, entry.Index)
		data = binary.LittleEndian.AppendUint64(data, entry.Term)
		data = binary.LittleEndian.AppendUint32(data, uint32(len(entry.Data)))
		data = append(data, entry.Data...)
	}
	return data
}

// decodeRaftUpdate 只解析有界结构，状态转换约束统一由 prepare 校验。
// 负载暂借用本次 WAL 回调的输入，prepare 在状态保留前复制。
func decodeRaftUpdate(data []byte) (RaftUpdate, error) {
	if len(data) < raftUpdateHeaderBytes || len(data) > maxJournalRecordBytes || string(data[:8]) != raftRecordMagic || data[8] != raftUpdateRecord {
		return RaftUpdate{}, ErrInvalidRaftState
	}
	update := RaftUpdate{HardState: RaftHardState{Term: binary.LittleEndian.Uint64(data[9:]), Vote: binary.LittleEndian.Uint64(data[17:]), Commit: binary.LittleEndian.Uint64(data[25:])}}
	count := binary.LittleEndian.Uint32(data[33:])
	data = data[raftUpdateHeaderBytes:]
	if count > maxRaftEntries || uint64(count)*raftEntryHeaderBytes > uint64(len(data)) {
		return RaftUpdate{}, ErrInvalidRaftState
	}
	update.Entries = make([]RaftEntry, 0, count)
	for range count {
		if len(data) < raftEntryHeaderBytes {
			return RaftUpdate{}, ErrInvalidRaftState
		}
		index := binary.LittleEndian.Uint64(data)
		term := binary.LittleEndian.Uint64(data[8:])
		length := uint64(binary.LittleEndian.Uint32(data[16:]))
		data = data[raftEntryHeaderBytes:]
		if length > maxRaftEntryBytes || length > uint64(len(data)) {
			return RaftUpdate{}, ErrInvalidRaftState
		}
		update.Entries = append(update.Entries, RaftEntry{Index: index, Term: term, Data: data[:length:length]})
		data = data[length:]
	}
	if len(data) != 0 {
		return RaftUpdate{}, ErrInvalidRaftState
	}
	return update, nil
}
