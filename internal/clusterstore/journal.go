package clusterstore

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
)

const (
	journalMagic          = "VIVIDWL1"
	journalHeaderBytes    = 8 + 16 + sha256.Size
	journalRecordHeader   = 16
	maxJournalRecordBytes = 4 << 20
	maxJournalBytes       = 256 << 20 // 分段与压缩接入前限制单文件增长
)

var (
	ErrInvalidJournal     = errors.New("invalid coordination journal")
	ErrJournalInUse       = errors.New("coordination journal is already in use")
	ErrJournalClosed      = errors.New("coordination journal is closed")
	ErrJournalFailed      = errors.New("coordination journal requires recovery after I/O failure")
	ErrJournalCapacity    = errors.New("coordination journal capacity exceeded")
	ErrJournalUnsupported = errors.New("coordination journal is unsupported on this platform")
	journalCRCTable       = crc32.MakeTable(crc32.Castagnoli)
)

type journalFile interface {
	io.ReaderAt
	io.WriterAt
	Sync() error
	Truncate(int64) error
	Close() error
}

// Journal 串行保存不透明的持久化批次，序号与 Raft 日志位置无关。
// 不判断多数派提交，也不自动应用状态机；使用后不可复制，所有操作须串行执行。
// 校验链用于发现损坏，不提供认证；底层文件系统须支持文件锁及刷盘语义。
type Journal struct {
	file      journalFile
	sequence  uint64
	offset    int64
	digest    [sha256.Size]byte
	buffer    []byte
	failed    bool
	recovered int64
}

// CreateJournal 在已有目录内创建新日志，目标存在时拒绝覆盖。
// 文件头刷盘后发布文件名；失败时目标仍可能已建立，须显式检查，不能自动重新初始化。
func CreateJournal(path string) (*Journal, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".vivid-journal-*")
	if err != nil {
		return nil, err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	var header [journalHeaderBytes]byte
	copy(header[:], journalMagic)
	if _, err = rand.Read(header[8:24]); err == nil {
		digest := sha256.Sum256(header[:24])
		copy(header[24:], digest[:])
		err = writeJournalAt(file, header[:], 0)
	}
	if err == nil {
		err = file.Sync()
	}
	err = errors.Join(err, file.Close())
	if err != nil {
		return nil, err
	}
	if err = publishJournal(temporary, path); err != nil {
		return nil, err
	}
	// Windows 发布前须关闭临时句柄；发布后统一通过锁竞争取得所有权。
	return OpenJournal(path, nil)
}

// OpenJournal 只打开已有日志，不把缺失、空白或损坏文件视为新节点。
// 回放按完整批次进行，负载归回调所有；任何恢复错误都要求丢弃回调已构造的状态。
// 仅截断未写完整的尾记录，完整记录损坏则保留原文件并报错。恢复完成前再次刷盘。
func OpenJournal(path string, replay func(uint64, []byte) error) (*Journal, error) {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	if err = lockJournal(file); err == nil {
		var info os.FileInfo
		info, err = file.Stat()
		if err == nil {
			if !info.Mode().IsRegular() || info.Size() < journalHeaderBytes || info.Size() > maxJournalBytes {
				err = ErrInvalidJournal
			} else {
				journal := &Journal{file: file, offset: journalHeaderBytes}
				if err = journal.recover(info.Size(), replay); err == nil {
					return journal, nil
				}
			}
		}
	}
	return nil, errors.Join(err, file.Close())
}

// Sequence 返回本地最后一条完整批次的序号，不表示 Raft 的提交位置。
func (j *Journal) Sequence() uint64 { return j.sequence }

// RecoveredTailBytes 返回本次打开时截断的尾部字节，便于上层报告恢复情况。
func (j *Journal) RecoveredTailBytes() int64 { return j.recovered }

// Append 在完整写入并 Sync 成功后返回序号，负载可包含一组需要共同恢复的更新。
// 调用期间负载不可修改；错误不代表批次一定不存在，I/O 失败后只允许关闭并重新恢复。
func (j *Journal) Append(payload []byte) (uint64, error) {
	if j.file == nil {
		return 0, ErrJournalClosed
	}
	if j.failed {
		return 0, ErrJournalFailed
	}
	frameBytes := int64(journalRecordHeader) + int64(len(payload)) + sha256.Size
	if len(payload) > maxJournalRecordBytes || j.sequence == ^uint64(0) || frameBytes > maxJournalBytes-j.offset {
		return 0, ErrJournalCapacity
	}
	if int64(cap(j.buffer)) < frameBytes {
		j.buffer = make([]byte, int(frameBytes))
	}
	frame := j.buffer[:int(frameBytes)]
	binary.LittleEndian.PutUint64(frame, j.sequence+1)
	binary.LittleEndian.PutUint32(frame[8:], uint32(len(payload)))
	binary.LittleEndian.PutUint32(frame[12:], crc32.Checksum(frame[:12], journalCRCTable))
	copy(frame[journalRecordHeader:], payload)
	digest := journalDigest(j.digest, frame[:journalRecordHeader], payload)
	copy(frame[journalRecordHeader+len(payload):], digest[:])
	err := writeJournalAt(j.file, frame, j.offset)
	if err == nil {
		err = j.file.Sync()
	}
	if err != nil {
		j.failed = true
		return 0, err
	}
	j.offset += frameBytes
	j.sequence++
	j.digest = digest
	return j.sequence, nil
}

// Close 释放句柄及进程间锁，不将先前失败的批次转为成功。
func (j *Journal) Close() error {
	if j.file == nil {
		return nil
	}
	file := j.file
	j.file = nil
	j.buffer = nil
	return file.Close()
}

func (j *Journal) recover(size int64, replay func(uint64, []byte) error) error {
	var fileHeader [journalHeaderBytes]byte
	if err := readJournalAt(j.file, fileHeader[:], 0); err != nil {
		return err
	}
	j.digest = sha256.Sum256(fileHeader[:24])
	if string(fileHeader[:8]) != journalMagic || !bytes.Equal(j.digest[:], fileHeader[24:]) {
		return ErrInvalidJournal
	}
	var header [journalRecordHeader]byte
	for j.offset < size {
		if size-j.offset < journalRecordHeader {
			break
		}
		if err := readJournalAt(j.file, header[:], j.offset); err != nil {
			return err
		}
		sequence := binary.LittleEndian.Uint64(header[:])
		length := binary.LittleEndian.Uint32(header[8:])
		if crc32.Checksum(header[:12], journalCRCTable) != binary.LittleEndian.Uint32(header[12:]) || sequence == 0 || sequence != j.sequence+1 || length > maxJournalRecordBytes {
			return fmt.Errorf("%w at offset %d", ErrInvalidJournal, j.offset)
		}
		frameBytes := int64(journalRecordHeader) + int64(length) + sha256.Size
		if frameBytes > size-j.offset {
			break
		}
		body := make([]byte, int(length)+sha256.Size)
		if err := readJournalAt(j.file, body, j.offset+journalRecordHeader); err != nil {
			return err
		}
		digest := journalDigest(j.digest, header[:], body[:length])
		if !bytes.Equal(digest[:], body[length:]) {
			return fmt.Errorf("%w at offset %d", ErrInvalidJournal, j.offset)
		}
		if replay != nil {
			if err := replay(sequence, body[:length:length]); err != nil {
				return fmt.Errorf("replay journal record %d: %w", sequence, err)
			}
		}
		j.sequence = sequence
		j.digest = digest
		j.offset += frameBytes
	}
	j.recovered = size - j.offset
	if j.recovered != 0 {
		if err := j.file.Truncate(j.offset); err != nil {
			return err
		}
	}
	return j.file.Sync()
}

func journalDigest(previous [sha256.Size]byte, header, payload []byte) [sha256.Size]byte {
	hash := sha256.New()
	hash.Write(previous[:])
	hash.Write(header)
	hash.Write(payload)
	var digest [sha256.Size]byte
	hash.Sum(digest[:0])
	return digest
}

func writeJournalAt(file io.WriterAt, data []byte, offset int64) error {
	n, err := file.WriteAt(data, offset)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	return err
}

func readJournalAt(file io.ReaderAt, data []byte, offset int64) error {
	n, err := file.ReadAt(data, offset)
	if err == nil && n != len(data) {
		err = io.ErrUnexpectedEOF
	}
	return err
}
