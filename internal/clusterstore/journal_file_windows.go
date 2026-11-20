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
	from, err := windows.UTF16PtrFromString(temporary)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	// 不设置 REPLACE_EXISTING，避免覆盖已有节点的数据；等待文件名移动完成。
	return windows.MoveFileEx(from, to, windows.MOVEFILE_WRITE_THROUGH)
}
