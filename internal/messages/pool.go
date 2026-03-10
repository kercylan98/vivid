package messages

import "sync"

var readerPool = sync.Pool{
	New: func() interface{} {
		return NewReader(nil)
	},
}

var writerPool = sync.Pool{
	New: func() interface{} {
		return NewWriter()
	},
}

func NewWriterFromPool() *Writer {
	w := writerPool.Get().(*Writer)
	return w
}

func ReleaseWriterToPool(w *Writer) {
	w.Reset()
	writerPool.Put(w)
}

func NewReaderFromPool(data []byte) *Reader {
	r := readerPool.Get().(*Reader)
	r.buf = data
	return r
}

func ReleaseReaderToPool(r *Reader) {
	r.Reset(nil)
	readerPool.Put(r)
}
