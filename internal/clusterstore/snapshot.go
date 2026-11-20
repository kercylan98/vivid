package clusterstore

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"sort"
)

const (
	snapshotVersion  = 1
	maxSnapshotBytes = 64 << 20
	snapshotHeader   = "VIVIDCS1"
)

var ErrInvalidSnapshot = errors.New("invalid coordination snapshot")

// Snapshot 返回独立且顺序固定的检查点，包含应用位置；不写磁盘，也不保证断电恢复。
// 校验和只用于发现损坏，不提供认证。后续持久化层负责日志、刷盘和原子安装。
func (s *Store) Snapshot() []byte {
	keys := make([]string, 0, len(s.records))
	for key := range s.records {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	headerSize := len(snapshotHeader) + sha256.Size
	data := make([]byte, headerSize, headerSize+16+len(keys)*16+s.bytes)
	copy(data, snapshotHeader)
	data = binary.LittleEndian.AppendUint32(data, snapshotVersion)
	data = binary.LittleEndian.AppendUint64(data, s.index)
	data = binary.LittleEndian.AppendUint32(data, uint32(len(keys)))
	for _, key := range keys {
		record := s.records[key]
		data = binary.LittleEndian.AppendUint32(data, uint32(len(key)))
		data = binary.LittleEndian.AppendUint32(data, uint32(len(record.Value)))
		data = binary.LittleEndian.AppendUint64(data, record.Revision)
		data = append(data, key...)
		data = append(data, record.Value...)
	}
	checksum := sha256.Sum256(data[headerSize:])
	copy(data[len(snapshotHeader):headerSize], checksum[:])
	return data
}

// Restore 校验后构造独立状态机，不覆盖正在运行的实例。
// 安装资格及其共识组身份由后续共识层核对，不能借此绕过已提交日志或回退隔离令牌。
func Restore(data []byte) (*Store, error) {
	headerSize := len(snapshotHeader) + sha256.Size
	if len(data) < headerSize+16 || len(data) > maxSnapshotBytes || string(data[:len(snapshotHeader)]) != snapshotHeader {
		return nil, ErrInvalidSnapshot
	}
	payload := data[headerSize:]
	checksum := sha256.Sum256(payload)
	if !bytes.Equal(data[len(snapshotHeader):headerSize], checksum[:]) {
		return nil, ErrInvalidSnapshot
	}
	version := binary.LittleEndian.Uint32(payload)
	index := binary.LittleEndian.Uint64(payload[4:])
	count := binary.LittleEndian.Uint32(payload[12:])
	payload = payload[16:]
	if version != snapshotVersion || count > maxEntries || uint64(count)*16 > uint64(len(payload)) {
		return nil, ErrInvalidSnapshot
	}
	store := &Store{index: index}
	if count > 0 {
		store.records = make(map[string]Record, count)
	}
	for range count {
		if len(payload) < 16 {
			return nil, ErrInvalidSnapshot
		}
		keyBytes := uint64(binary.LittleEndian.Uint32(payload))
		valueBytes := uint64(binary.LittleEndian.Uint32(payload[4:]))
		revision := binary.LittleEndian.Uint64(payload[8:])
		payload = payload[16:]
		if keyBytes == 0 || keyBytes > maxKeyBytes || valueBytes > maxValueBytes || keyBytes+valueBytes > uint64(len(payload)) || revision == 0 || revision > index {
			return nil, ErrInvalidSnapshot
		}
		key := string(payload[:keyBytes])
		if !validKey(key) {
			return nil, ErrInvalidSnapshot
		}
		if _, exists := store.records[key]; exists {
			return nil, ErrInvalidSnapshot
		}
		store.bytes += int(keyBytes + valueBytes)
		if store.bytes > maxStateBytes {
			return nil, ErrInvalidSnapshot
		}
		store.records[key] = Record{Value: bytes.Clone(payload[keyBytes : keyBytes+valueBytes]), Revision: revision}
		payload = payload[keyBytes+valueBytes:]
	}
	if len(payload) != 0 {
		return nil, ErrInvalidSnapshot
	}
	return store, nil
}
