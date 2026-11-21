package clusterstore

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

func lockJournal(file *os.File) error {
	err := windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &windows.Overlapped{})
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return errors.Join(ErrJournalInUse, err)
	}
	return err
}

func publishJournal(temporary, path string) error {
	return moveJournal(temporary, path, windows.MOVEFILE_WRITE_THROUGH)
}

func replaceJournal(temporary, path string) error {
	return moveJournal(temporary, path, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}

func syncJournalDirectory(string) error { return nil } // 发布使用 MOVEFILE_WRITE_THROUGH，删除残留不影响恢复入口

func moveJournal(temporary, path string, flags uint32) error {
	from, err := windows.UTF16PtrFromString(temporary)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(from, to, flags)
}
