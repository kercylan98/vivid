package conn

import "github.com/kercylan98/vivid/pkg/vivid"

var (
	_ vivid.ConnReaderOptions        = (*readerOptions)(nil)
	_ vivid.ConnReaderOptionsFetcher = (*readerOptions)(nil)
)

func ReaderOptions() vivid.ConnReaderOptions {
	return &readerOptions{}
}

type readerOptions struct {
}
