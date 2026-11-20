//go:build !windows && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd

package clusterstore

import "os"

func lockJournal(*os.File) error { return ErrJournalUnsupported }

func publishJournal(string, string) error { return ErrJournalUnsupported }
