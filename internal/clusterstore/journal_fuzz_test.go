package clusterstore

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

type recoveryJournalFile struct {
	*bytes.Reader
	truncated int64
	syncs     int
}

func (*recoveryJournalFile) WriteAt([]byte, int64) (int, error) { return 0, io.ErrClosedPipe }
func (*recoveryJournalFile) Close() error                       { return nil }
func (f *recoveryJournalFile) Truncate(size int64) error        { f.truncated = size; return nil }
func (f *recoveryJournalFile) Sync() error                      { f.syncs++; return nil }

func FuzzJournal_Recovery(f *testing.F) {
	image := journalImage(f, []byte("first batch"), []byte("second batch"))
	f.Add([]byte{})
	f.Add(image)
	f.Add(image[:journalHeaderBytes])
	f.Add(image[:len(image)-1])
	f.Add(image[:journalHeaderBytes+journalRecordHeader-1])
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < journalHeaderBytes || len(data) > maxJournalRecordBytes+1024 {
			return
		}
		file := &recoveryJournalFile{Reader: bytes.NewReader(data), truncated: -1}
		journal := &Journal{file: file, offset: journalHeaderBytes}
		var last uint64
		err := journal.recover(int64(len(data)), func(sequence uint64, payload []byte) error {
			require.Equal(t, last+1, sequence)
			require.LessOrEqual(t, len(payload), maxJournalRecordBytes)
			last = sequence
			clear(payload) // 回调修改不能影响下一批次的校验链
			return nil
		})
		if err != nil {
			require.ErrorIs(t, err, ErrInvalidJournal)
			require.EqualValues(t, -1, file.truncated, "损坏不能改变文件")
			require.Zero(t, file.syncs)
			return
		}
		require.Equal(t, last, journal.Sequence())
		require.EqualValues(t, 1, file.syncs)
		require.Equal(t, int64(len(data))-journal.offset, journal.RecoveredTailBytes())
		if journal.RecoveredTailBytes() > 0 {
			require.Equal(t, journal.offset, file.truncated)
		} else {
			require.EqualValues(t, -1, file.truncated)
		}
	})
}
