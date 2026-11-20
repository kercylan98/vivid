//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package clusterstore

import (
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func lockJournal(file *os.File) error {
	err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) {
		return errors.Join(ErrJournalInUse, err)
	}
	return err
}

func publishJournal(temporary, path string) error {
	// 同目录硬链接原子创建且不覆盖目标，发布后同步目录项。
	if err := os.Link(temporary, path); err != nil {
		return err
	}
	if err := os.Remove(temporary); err != nil {
		return err
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	err = directory.Sync()
	return errors.Join(err, directory.Close())
}
