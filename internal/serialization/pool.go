package serialization

import "sync"

var (
	readerPool = sync.Pool{
		New: func() any {
			return &Reader{}
		},
	}
	writerPool = sync.Pool{
		New: func() any {
			return &Writer{}
		},
	}
)
