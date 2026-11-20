package clusterstore

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func createJournal(t testing.TB) (*Journal, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "coordination.wal")
	journal, err := CreateJournal(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, journal.Close()) })
	return journal, path
}

func journalImage(t testing.TB, payloads ...[]byte) []byte {
	t.Helper()
	journal, path := createJournal(t)
	for i, payload := range payloads {
		sequence, err := journal.Append(payload)
		require.NoError(t, err)
		require.EqualValues(t, i+1, sequence)
	}
	require.NoError(t, journal.Close())
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}

func TestJournal_RoundTrip(t *testing.T) {
	journal, path := createJournal(t)
	payload := []byte("node-a")
	sequence, err := journal.Append(payload)
	require.NoError(t, err)
	require.EqualValues(t, 1, sequence)
	payload[0] = 'x'
	_, err = journal.Append([]byte("node-b"))
	require.NoError(t, err)
	_, err = journal.Append(nil)
	require.NoError(t, err)
	require.NoError(t, journal.Close())
	require.NoError(t, journal.Close())
	_, err = journal.Append(nil)
	require.ErrorIs(t, err, ErrJournalClosed)

	var recovered [][]byte
	reopened, err := OpenJournal(path, func(sequence uint64, payload []byte) error {
		require.EqualValues(t, len(recovered)+1, sequence)
		recovered = append(recovered, payload)
		return nil
	})
	require.NoError(t, err)
	defer reopened.Close()
	require.Equal(t, [][]byte{[]byte("node-a"), []byte("node-b"), {}}, recovered)
	require.EqualValues(t, 3, reopened.Sequence())
	require.Zero(t, reopened.RecoveredTailBytes())
	recovered[0][0] = 'y'
	sequence, err = reopened.Append([]byte("next"))
	require.NoError(t, err)
	require.EqualValues(t, 4, sequence, "回调修改负载不能影响后续日志校验链")
	require.NoError(t, reopened.Close())
	check, err := OpenJournal(path, nil)
	require.NoError(t, err)
	require.EqualValues(t, 4, check.Sequence())
	require.NoError(t, check.Close())
}

func TestJournal_Exclusive(t *testing.T) {
	journal, path := createJournal(t)
	other, err := OpenJournal(path, nil)
	require.ErrorIs(t, err, ErrJournalInUse)
	require.Nil(t, other)
	other, err = CreateJournal(path)
	require.Error(t, err, "创建不能覆盖已有文件")
	require.Nil(t, other)
	// 同一 inode 的其他名称也必须受到锁保护。
	alias := filepath.Join(filepath.Dir(path), "alias.wal")
	require.NoError(t, os.Link(path, alias))
	other, err = OpenJournal(alias, nil)
	require.ErrorIs(t, err, ErrJournalInUse)
	require.Nil(t, other)
	require.NoError(t, journal.Close())
	other, err = OpenJournal(path, nil)
	require.NoError(t, err)
	require.Zero(t, other.Sequence(), "创建失败不能覆盖原文件头")
	require.NoError(t, other.Close())
	files, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".vivid-journal-*"))
	require.NoError(t, err)
	require.Empty(t, files)
}

func TestJournal_TruncatedTail(t *testing.T) {
	first, second := []byte("first batch"), []byte("second batch")
	image := journalImage(t, first, second)
	firstEnd := journalHeaderBytes + journalRecordHeader + len(first) + 32
	path := filepath.Join(t.TempDir(), "truncated.wal")
	for size := journalHeaderBytes; size <= len(image); size++ {
		require.NoError(t, os.WriteFile(path, image[:size], 0600))
		var recovered [][]byte
		journal, err := OpenJournal(path, func(_ uint64, payload []byte) error {
			recovered = append(recovered, payload)
			return nil
		})
		require.NoError(t, err, "尾部大小 %d", size)
		validEnd, count := journalHeaderBytes, 0
		if size >= firstEnd {
			validEnd, count = firstEnd, 1
			require.Equal(t, first, recovered[0])
		}
		if size == len(image) {
			validEnd, count = len(image), 2
			require.Equal(t, second, recovered[1])
		}
		require.Len(t, recovered, count)
		require.EqualValues(t, size-validEnd, journal.RecoveredTailBytes())
		sequence, err := journal.Append([]byte("after recovery"))
		require.NoError(t, err)
		require.EqualValues(t, count+1, sequence)
		require.NoError(t, journal.Close())
		check, err := OpenJournal(path, nil)
		require.NoError(t, err)
		require.Equal(t, sequence, check.Sequence())
		require.Zero(t, check.RecoveredTailBytes())
		require.NoError(t, check.Close())
	}
}

func TestJournal_Corruption(t *testing.T) {
	image := journalImage(t, []byte("first"), []byte("second"))
	path := filepath.Join(t.TempDir(), "invalid.wal")
	reject := func(data []byte) {
		t.Helper()
		require.NoError(t, os.WriteFile(path, data, 0600))
		journal, err := OpenJournal(path, nil)
		require.ErrorIs(t, err, ErrInvalidJournal)
		require.Nil(t, journal)
		after, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, data, after, "完整记录损坏不能当作尾部残片截断")
	}
	for offset := range len(image) {
		data := bytes.Clone(image)
		data[offset] ^= 1
		reject(data)
	}
	for size := range journalHeaderBytes {
		reject(image[:size])
	}
	// 即使重算头校验，也要拒绝异常长度、重复及跳跃序号。
	for _, sequence := range []uint64{0, 2, ^uint64(0)} {
		data := bytes.Clone(image)
		binary.LittleEndian.PutUint64(data[journalHeaderBytes:], sequence)
		binary.LittleEndian.PutUint32(data[journalHeaderBytes+12:], crc32.Checksum(data[journalHeaderBytes:journalHeaderBytes+12], journalCRCTable))
		reject(data)
	}
	data := bytes.Clone(image)
	binary.LittleEndian.PutUint32(data[journalHeaderBytes+8:], ^uint32(0))
	binary.LittleEndian.PutUint32(data[journalHeaderBytes+12:], crc32.Checksum(data[journalHeaderBytes:journalHeaderBytes+12], journalCRCTable))
	reject(data)
	other := journalImage(t, []byte("first"), []byte("second"))
	reject(append(bytes.Clone(image[:journalHeaderBytes]), other[journalHeaderBytes:]...))
}

func TestJournal_ReplayFailure(t *testing.T) {
	image := journalImage(t, []byte("valid"), []byte("next"))
	image = append(image, 1, 2, 3) // 失败回调之后不能截断残片
	path := filepath.Join(t.TempDir(), "callback.wal")
	require.NoError(t, os.WriteFile(path, image, 0600))
	expected := errors.New("cannot restore state")
	journal, err := OpenJournal(path, func(uint64, []byte) error { return expected })
	require.ErrorIs(t, err, expected)
	require.Nil(t, journal)
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, image, after)
	journal, err = OpenJournal(path, nil)
	require.NoError(t, err, "恢复失败后必须释放锁")
	require.EqualValues(t, 3, journal.RecoveredTailBytes())
	require.NoError(t, journal.Close())
}

func TestJournal_Capacity(t *testing.T) {
	journal, path := createJournal(t)
	_, err := journal.Append(make([]byte, maxJournalRecordBytes+1))
	require.ErrorIs(t, err, ErrJournalCapacity)
	require.False(t, journal.failed)
	payload := bytes.Repeat([]byte{1}, maxJournalRecordBytes)
	_, err = journal.Append(payload)
	require.NoError(t, err, "允许正好达到单条上限")
	originalOffset := journal.offset
	journal.offset = maxJournalBytes - journalRecordHeader - 32
	_, err = journal.Append([]byte{1})
	require.ErrorIs(t, err, ErrJournalCapacity)
	journal.offset = originalOffset
	sequence := journal.sequence
	journal.sequence = ^uint64(0)
	_, err = journal.Append(nil)
	require.ErrorIs(t, err, ErrJournalCapacity, "序号不能回绕")
	journal.sequence = sequence
	_, err = journal.Append(nil)
	require.NoError(t, err, "预检拒绝不关闭后续合法写入")
	require.NoError(t, journal.Close())
	check, err := OpenJournal(path, func(sequence uint64, data []byte) error {
		if sequence == 1 {
			require.Equal(t, payload, data)
		}
		return nil
	})
	require.NoError(t, err)
	require.NoError(t, check.Close())
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	require.NoError(t, err)
	require.NoError(t, file.Truncate(maxJournalBytes+1))
	require.NoError(t, file.Close())
	check, err = OpenJournal(path, nil)
	require.ErrorIs(t, err, ErrInvalidJournal)
	require.Nil(t, check)
	check, err = OpenJournal(filepath.Join(t.TempDir(), "missing.wal"), nil)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.Nil(t, check)
}

type failingJournalFile struct {
	journalFile
	writeLimit  int
	writeErr    error
	syncErr     error
	readErr     error
	truncateErr error
	writes      int
	syncs       int
}

func (f *failingJournalFile) WriteAt(data []byte, offset int64) (int, error) {
	f.writes++
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	if f.writeLimit > 0 {
		return f.journalFile.WriteAt(data[:f.writeLimit], offset)
	}
	return f.journalFile.WriteAt(data, offset)
}

func (f *failingJournalFile) Sync() error {
	f.syncs++
	if f.syncErr != nil {
		return f.syncErr
	}
	return f.journalFile.Sync()
}

func (f *failingJournalFile) ReadAt(data []byte, offset int64) (int, error) {
	if f.readErr != nil {
		return 0, f.readErr
	}
	return f.journalFile.ReadAt(data, offset)
}

func (f *failingJournalFile) Truncate(size int64) error {
	if f.truncateErr != nil {
		return f.truncateErr
	}
	return f.journalFile.Truncate(size)
}

func TestJournal_WriteFailure(t *testing.T) {
	failure := errors.New("disk failure")
	for _, kind := range []string{"short_header", "short_body", "write", "sync"} {
		t.Run(kind, func(t *testing.T) {
			journal, path := createJournal(t)
			_, err := journal.Append([]byte("confirmed"))
			require.NoError(t, err)
			fault := &failingJournalFile{journalFile: journal.file}
			journal.file = fault
			expected := failure
			switch kind {
			case "short_header":
				fault.writeLimit, expected = 5, io.ErrShortWrite
			case "short_body":
				fault.writeLimit, expected = journalRecordHeader+1, io.ErrShortWrite
			case "write":
				fault.writeErr = failure
			case "sync":
				fault.syncErr = failure
			}
			_, err = journal.Append([]byte("uncertain"))
			require.ErrorIs(t, err, expected)
			require.EqualValues(t, 1, journal.Sequence())
			writes, syncs := fault.writes, fault.syncs
			_, err = journal.Append([]byte("retry"))
			require.ErrorIs(t, err, ErrJournalFailed)
			require.Equal(t, writes, fault.writes)
			require.Equal(t, syncs, fault.syncs)
			require.NoError(t, journal.Close())
			var payloads []string
			recovered, err := OpenJournal(path, func(_ uint64, payload []byte) error {
				payloads = append(payloads, string(payload))
				return nil
			})
			require.NoError(t, err)
			if kind == "sync" {
				require.Equal(t, []string{"confirmed", "uncertain"}, payloads, "失败批次可能完整存在，调用方不能直接重试业务")
			} else {
				require.Equal(t, []string{"confirmed"}, payloads)
			}
			require.NoError(t, recovered.Close())
		})
	}
}

func TestJournal_RecoveryIOFailure(t *testing.T) {
	failure := errors.New("disk failure")
	for _, kind := range []string{"read", "truncate", "sync"} {
		t.Run(kind, func(t *testing.T) {
			journal, _ := createJournal(t)
			_, err := journal.Append([]byte("confirmed"))
			require.NoError(t, err)
			_, err = journal.file.WriteAt([]byte{1}, journal.offset)
			require.NoError(t, err)
			fault := &failingJournalFile{journalFile: journal.file}
			switch kind {
			case "read":
				fault.readErr = failure
			case "truncate":
				fault.truncateErr = failure
			case "sync":
				fault.syncErr = failure
			}
			recovering := &Journal{file: fault, offset: journalHeaderBytes}
			require.ErrorIs(t, recovering.recover(journal.offset+1, nil), failure)
		})
	}
}

func TestJournal_ProcessHelper(t *testing.T) {
	path := os.Getenv("VIVID_JOURNAL_CRASH_PATH")
	if path == "" {
		return
	}
	journal, err := CreateJournal(path)
	require.NoError(t, err)
	_, err = journal.Append([]byte("confirmed"))
	require.NoError(t, err)
	if os.Getenv("VIVID_JOURNAL_CRASH_MODE") == "partial" {
		var header [journalRecordHeader]byte
		binary.LittleEndian.PutUint64(header[:], 2)
		binary.LittleEndian.PutUint32(header[8:], 100)
		binary.LittleEndian.PutUint32(header[12:], crc32.Checksum(header[:12], journalCRCTable))
		require.NoError(t, writeJournalAt(journal.file, append(header[:], []byte("partial")...), journal.offset))
	}
	fmt.Println("ready")
	<-time.After(time.Minute) // 由父进程强制终止，不执行 Close
	t.Fatal("parent did not terminate helper")
}

func TestJournal_ProcessCrash(t *testing.T) {
	for _, mode := range []string{"clean", "partial"} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "crash.wal")
			command := exec.Command(os.Args[0], "-test.run=^TestJournal_ProcessHelper$", "-test.timeout=30s")
			command.Env = append(os.Environ(), "VIVID_JOURNAL_CRASH_PATH="+path, "VIVID_JOURNAL_CRASH_MODE="+mode)
			stdout, err := command.StdoutPipe()
			require.NoError(t, err)
			var stderr bytes.Buffer
			command.Stderr = &stderr
			require.NoError(t, command.Start())
			t.Cleanup(func() {
				if command.ProcessState == nil {
					command.Process.Kill()
					command.Wait()
				}
			})
			ready := make(chan string, 1)
			go func() {
				line, _ := bufio.NewReader(stdout).ReadString('\n')
				ready <- line
			}()
			select {
			case line := <-ready:
				require.Equal(t, "ready\n", line)
			case <-time.After(10 * time.Second):
				t.Fatal("helper did not become ready")
			}
			locked, err := OpenJournal(path, nil)
			require.ErrorIs(t, err, ErrJournalInUse)
			require.Nil(t, locked)
			require.NoError(t, command.Process.Kill())
			require.Error(t, command.Wait())
			var payloads []string
			journal, err := OpenJournal(path, func(_ uint64, payload []byte) error {
				payloads = append(payloads, string(payload))
				return nil
			})
			require.NoError(t, err, stderr.String())
			require.Equal(t, []string{"confirmed"}, payloads)
			if mode == "partial" {
				require.Positive(t, journal.RecoveredTailBytes())
			} else {
				require.Zero(t, journal.RecoveredTailBytes())
			}
			sequence, err := journal.Append([]byte("after crash"))
			require.NoError(t, err)
			require.EqualValues(t, 2, sequence)
			require.NoError(t, journal.Close())
		})
	}
}

func BenchmarkJournal_Append(b *testing.B) {
	for _, size := range []int{64, 4096} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			path := filepath.Join(b.TempDir(), "bench.wal")
			journal, err := CreateJournal(path)
			if err != nil {
				b.Fatal(err)
			}
			defer func() { journal.Close() }()
			payload := make([]byte, size)
			b.ReportAllocs()
			b.SetBytes(int64(size))
			for b.Loop() {
				if journal.offset+int64(journalRecordHeader+size+32) > maxJournalBytes {
					// 仅测追加与刷盘，排除达到文件上限后的测试文件重建。
					b.StopTimer()
					if err := journal.Close(); err != nil {
						b.Fatal(err)
					}
					if err := os.Remove(path); err != nil {
						b.Fatal(err)
					}
					next, err := CreateJournal(path)
					if err != nil {
						b.Fatal(err)
					}
					journal = next
					b.StartTimer()
				}
				if _, err := journal.Append(payload); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkJournal_Recover(b *testing.B) {
	journal, path := createJournal(b)
	for range 1000 {
		if _, err := journal.Append(make([]byte, 64)); err != nil {
			b.Fatal(err)
		}
	}
	if err := journal.Close(); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		recovered, err := OpenJournal(path, nil)
		if err != nil {
			b.Fatal(err)
		}
		if recovered.Sequence() != 1000 {
			b.Fatal(recovered.Sequence())
		}
		if err := recovered.Close(); err != nil {
			b.Fatal(err)
		}
	}
}
