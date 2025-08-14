package vivid

import "errors"

func newWatchInfo(ref ActorRef) *watchInfo {
	return &watchInfo{
		ref: ref,
	}
}

type watchInfo struct {
	ref    ActorRef
	errors []error
}

func (w *watchInfo) recordError(err error) {
	w.errors = append(w.errors, err)
}

func (w *watchInfo) reset() {
	w.errors = w.errors[:0]
}

func (w *watchInfo) getErrors() []error {
	return w.errors
}

func (w *watchInfo) getError() error {
	if len(w.errors) > 0 {
		return errors.Join(w.errors...)
	}
	return nil
}
