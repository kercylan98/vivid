package vivid

import (
	"math"
	"testing"
)

func TestReader_checkRead(t *testing.T) {
	var testCases = []struct {
		name   string
		buf    []byte
		offset int
		size   int
		want   bool
	}{
		{name: "checkRead", buf: []byte{0x00, 0x00, 0x00, 0x04, 0x01, 0x02, 0x03, 0x04}, offset: 0, size: 1, want: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			reader := newReader(testCase.buf)
			reader.offset = testCase.offset
			got := reader.checkRead(testCase.size)
			if got != testCase.want {
				t.Errorf("checkRead(%d) = %v, want %v", testCase.size, got, testCase.want)
			}
		})
	}
}

// testReaderGeneric 通用的读取器测试函数
func testReaderGeneric[T comparable](t *testing.T, testName string, writeFunc func(*writer, T), readFunc func(*reader) T, testCases [][]T) {
	for i, expects := range testCases {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range expects {
				writeFunc(w, expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range expects {
				got := readFunc(reader)
				if got != expect {
					t.Errorf("测试用例 %d: %s() = %v, want %v", i, testName, got, expect)
				}
			}
		})
	}
}

func TestReader_readUint8(t *testing.T) {
	testCases := [][]uint8{
		{1},
		{1, 2, 3, 4},
	}
	testReaderGeneric(t, t.Name(),
		func(w *writer, value uint8) {
			w.writeUint8(value)
		},
		func(r *reader) uint8 {
			return r.readUint8()
		},
		testCases,
	)
}

func TestReader_readUint16(t *testing.T) {
	testCases := [][]uint16{
		{1},
		{1, 2, 3, 4},
	}
	testReaderGeneric(t, t.Name(),
		func(w *writer, value uint16) {
			w.writeUint16(value)
		},
		func(r *reader) uint16 {
			return r.readUint16()
		},
		testCases,
	)
}

func TestReader_readUint32(t *testing.T) {
	testCases := [][]uint32{
		{1},
		{1, 2, 3, 4},
	}
	testReaderGeneric(t, t.Name(),
		func(w *writer, value uint32) {
			w.writeUint32(value)
		},
		func(r *reader) uint32 {
			return r.readUint32()
		},
		testCases,
	)
}

func TestReader_readUint64(t *testing.T) {
	testCases := [][]uint64{
		{1},
		{1, 2, 3, 4},
		{math.MaxUint64, 2, 3, 4},
	}
	testReaderGeneric(t, t.Name(),
		func(w *writer, value uint64) {
			w.writeUint64(value)
		},
		func(r *reader) uint64 {
			return r.readUint64()
		},
		testCases,
	)
}

func TestReader_readInt8(t *testing.T) {
	testCases := [][]int8{
		{1},
		{1, 2, 3, 4},
	}
	testReaderGeneric(t, t.Name(),
		func(w *writer, value int8) {
			w.writeInt8(value)
		},
		func(r *reader) int8 {
			return r.readInt8()
		},
		testCases,
	)
}

func TestReader_readInt16(t *testing.T) {
	testCases := [][]int16{
		{1},
		{1, 2, 3, 4},
	}
	testReaderGeneric(t, t.Name(),
		func(w *writer, value int16) {
			w.writeInt16(value)
		},
		func(r *reader) int16 {
			return r.readInt16()
		},
		testCases,
	)
}

func TestReader_readInt32(t *testing.T) {
	testCases := [][]int32{
		{1},
		{1, 2, 3, 4},
	}
	testReaderGeneric(t, t.Name(),
		func(w *writer, value int32) {
			w.writeInt32(value)
		},
		func(r *reader) int32 {
			return r.readInt32()
		},
		testCases,
	)
}

func TestReader_readInt64(t *testing.T) {
	testCases := [][]int64{
		{1},
		{1, 2, 3, 4},
	}
	testReaderGeneric(t, t.Name(),
		func(w *writer, value int64) {
			w.writeInt64(value)
		},
		func(r *reader) int64 {
			return r.readInt64()
		},
		testCases,
	)
}

func TestReader_readFloat32(t *testing.T) {
	testCases := [][]float32{
		{1},
		{1, 2, 3, 4},
	}
	testReaderGeneric(t, t.Name(),
		func(w *writer, value float32) {
			w.writeFloat32(value)
		},
		func(r *reader) float32 {
			return r.readFloat32()
		},
		testCases,
	)
}

func TestReader_readFloat64(t *testing.T) {
	testCases := [][]float64{
		{1},
		{1, 2, 3, 4},
	}
	testReaderGeneric(t, t.Name(),
		func(w *writer, value float64) {
			w.writeFloat64(value)
		},
		func(r *reader) float64 {
			return r.readFloat64()
		},
		testCases,
	)
}

func TestReader_readBool(t *testing.T) {
	testCases := [][]bool{
		{true},
		{true, false, true, false},
	}
	testReaderGeneric(t, t.Name(),
		func(w *writer, value bool) {
			w.writeBool(value)
		},
		func(r *reader) bool {
			return r.readBool()
		},
		testCases,
	)
}

func TestReader_readString(t *testing.T) {
	testCases := [][]string{
		{"hello"},
		{"hello", "world", "foo", "bar"},
	}
	testReaderGeneric(t, t.Name(),
		func(w *writer, value string) {
			w.writeString(value)
		},
		func(r *reader) string {
			return r.readString()
		},
		testCases,
	)
}

func TestReader_readStrings(t *testing.T) {
	var testCases = []struct {
		name    string
		expects [][]string
	}{
		{name: "readStrings", expects: [][]string{
			{"hello"},
			{"hello", "world", "foo", "bar"},
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeStrings(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				got := reader.readStrings()
				if len(got) != len(expect) {
					t.Errorf("readStrings() = %v, want %v", got, expect)
					continue
				}
				for i := range got {
					if got[i] != expect[i] {
						t.Errorf("readStrings() = %v, want %v", got, expect)
						break
					}
				}
			}
		})
	}
}

func TestReader_readUint8s(t *testing.T) {
	var testCases = []struct {
		name    string
		expects [][]uint8
	}{
		{name: "readUint8s", expects: [][]uint8{
			{1},
			{1, 2, 3, 4},
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeUint8s(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				got := reader.readUint8s()
				if len(got) != len(expect) {
					t.Errorf("readUint8s() = %v, want %v", got, expect)
					continue
				}
				for i := range got {
					if got[i] != expect[i] {
						t.Errorf("readUint8s() = %v, want %v", got, expect)
						break
					}
				}
			}
		})
	}
}

func TestReader_readUint16s(t *testing.T) {
	var testCases = []struct {
		name    string
		expects [][]uint16
	}{
		{name: "readUint16s", expects: [][]uint16{
			{1},
			{1, 2, 3, 4},
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeUint16s(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				got := reader.readUint16s()
				if len(got) != len(expect) {
					t.Errorf("readUint16s() = %v, want %v", got, expect)
					continue
				}
				for i := range got {
					if got[i] != expect[i] {
						t.Errorf("readUint16s() = %v, want %v", got, expect)
						break
					}
				}
			}
		})
	}
}

func TestReader_readUint32s(t *testing.T) {
	var testCases = []struct {
		name    string
		expects [][]uint32
	}{
		{name: "readUint32s", expects: [][]uint32{
			{1},
			{1, 2, 3, 4},
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeUint32s(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				got := reader.readUint32s()
				if len(got) != len(expect) {
					t.Errorf("readUint32s() = %v, want %v", got, expect)
					continue
				}
				for i := range got {
					if got[i] != expect[i] {
						t.Errorf("readUint32s() = %v, want %v", got, expect)
						break
					}
				}
			}
		})
	}
}

func TestReader_readUint64s(t *testing.T) {
	var testCases = []struct {
		name    string
		expects [][]uint64
	}{
		{name: "readUint64s", expects: [][]uint64{
			{1},
			{1, 2, 3, 4},
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeUint64s(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				got := reader.readUint64s()
				if len(got) != len(expect) {
					t.Errorf("readUint64s() = %v, want %v", got, expect)
					continue
				}
				for i := range got {
					if got[i] != expect[i] {
						t.Errorf("readUint64s() = %v, want %v", got, expect)
						break
					}
				}
			}
		})
	}
}

func TestReader_readInt8s(t *testing.T) {
	var testCases = []struct {
		name    string
		expects [][]int8
	}{
		{name: "readInt8s", expects: [][]int8{
			{1},
			{1, 2, 3, 4},
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeInt8s(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				got := reader.readInt8s()
				if len(got) != len(expect) {
					t.Errorf("readInt8s() = %v, want %v", got, expect)
					continue
				}
				for i := range got {
					if got[i] != expect[i] {
						t.Errorf("readInt8s() = %v, want %v", got, expect)
						break
					}
				}
			}
		})
	}
}

func TestReader_readInt16s(t *testing.T) {
	var testCases = []struct {
		name    string
		expects [][]int16
	}{
		{name: "readInt16s", expects: [][]int16{
			{1},
			{1, 2, 3, 4},
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeInt16s(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				got := reader.readInt16s()
				if len(got) != len(expect) {
					t.Errorf("readInt16s() = %v, want %v", got, expect)
					continue
				}
				for i := range got {
					if got[i] != expect[i] {
						t.Errorf("readInt16s() = %v, want %v", got, expect)
						break
					}
				}
			}
		})
	}
}

func TestReader_readInt32s(t *testing.T) {
	var testCases = []struct {
		name    string
		expects [][]int32
	}{
		{name: "readInt32s", expects: [][]int32{
			{1},
			{1, 2, 3, 4},
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeInt32s(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				got := reader.readInt32s()
				if len(got) != len(expect) {
					t.Errorf("readInt32s() = %v, want %v", got, expect)
					continue
				}
				for i := range got {
					if got[i] != expect[i] {
						t.Errorf("readInt32s() = %v, want %v", got, expect)
						break
					}
				}
			}
		})
	}
}

func TestReader_readInt64s(t *testing.T) {
	var testCases = []struct {
		name    string
		expects [][]int64
	}{
		{name: "readInt64s", expects: [][]int64{
			{1},
			{1, 2, 3, 4},
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeInt64s(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				got := reader.readInt64s()
				if len(got) != len(expect) {
					t.Errorf("readInt64s() = %v, want %v", got, expect)
					continue
				}
				for i := range got {
					if got[i] != expect[i] {
						t.Errorf("readInt64s() = %v, want %v", got, expect)
						break
					}
				}
			}
		})
	}
}

func TestReader_readFloat32s(t *testing.T) {
	var testCases = []struct {
		name    string
		expects [][]float32
	}{
		{name: "readFloat32s", expects: [][]float32{
			{1},
			{1, 2, 3, 4},
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeFloat32s(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				got := reader.readFloat32s()
				if len(got) != len(expect) {
					t.Errorf("readFloat32s() = %v, want %v", got, expect)
					continue
				}
				for i := range got {
					if got[i] != expect[i] {
						t.Errorf("readFloat32s() = %v, want %v", got, expect)
						break
					}
				}
			}
		})
	}
}

func TestReader_readFloat64s(t *testing.T) {
	var testCases = []struct {
		name    string
		expects [][]float64
	}{
		{name: "readFloat64s", expects: [][]float64{
			{1},
			{1, 2, 3, 4},
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeFloat64s(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				got := reader.readFloat64s()
				if len(got) != len(expect) {
					t.Errorf("readFloat64s() = %v, want %v", got, expect)
					continue
				}
				for i := range got {
					if got[i] != expect[i] {
						t.Errorf("readFloat64s() = %v, want %v", got, expect)
						break
					}
				}
			}
		})
	}
}

func TestReader_readBools(t *testing.T) {
	var testCases = []struct {
		name    string
		expects [][]bool
	}{
		{name: "readBools", expects: [][]bool{
			{true},
			{true, false, true, false},
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeBools(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				got := reader.readBools()
				if len(got) != len(expect) {
					t.Errorf("readBools() = %v, want %v", got, expect)
					continue
				}
				for i := range got {
					if got[i] != expect[i] {
						t.Errorf("readBools() = %v, want %v", got, expect)
						break
					}
				}
			}
		})
	}
}

func TestReader_readBytes(t *testing.T) {
	var testCases = []struct {
		name    string
		expects [][]byte
	}{
		{name: "readBytes", expects: [][]byte{
			{1},
			{1, 2, 3, 4},
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeBytes(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				got := reader.readBytes()
				if len(got) != len(expect) {
					t.Errorf("readBytes() = %v, want %v", got, expect)
					continue
				}
				for i := range got {
					if got[i] != expect[i] {
						t.Errorf("readBytes() = %v, want %v", got, expect)
						break
					}
				}
			}
		})
	}
}

func TestReader_readUint8To(t *testing.T) {
	var testCases = []struct {
		name    string
		expects []uint8
	}{
		{name: "readUint8To", expects: []uint8{1, 2, 3, 4}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeUint8(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				var got uint8
				reader.readUint8To(&got)
				if got != expect {
					t.Errorf("readUint8To() = %v, want %v", got, expect)
				}
			}
		})
	}
}

func TestReader_readUint16To(t *testing.T) {
	var testCases = []struct {
		name    string
		expects []uint16
	}{
		{name: "readUint16To", expects: []uint16{1, 2, 3, 4}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeUint16(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				var got uint16
				reader.readUint16To(&got)
				if got != expect {
					t.Errorf("readUint16To() = %v, want %v", got, expect)
				}
			}
		})
	}
}

func TestReader_readUint32To(t *testing.T) {
	var testCases = []struct {
		name    string
		expects []uint32
	}{
		{name: "readUint32To", expects: []uint32{1, 2, 3, 4}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeUint32(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				var got uint32
				reader.readUint32To(&got)
				if got != expect {
					t.Errorf("readUint32To() = %v, want %v", got, expect)
				}
			}
		})
	}
}

func TestReader_readUint64To(t *testing.T) {
	var testCases = []struct {
		name    string
		expects []uint64
	}{
		{name: "readUint64To", expects: []uint64{1, 2, 3, 4}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeUint64(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				var got uint64
				reader.readUint64To(&got)
				if got != expect {
					t.Errorf("readUint64To() = %v, want %v", got, expect)
				}
			}
		})
	}
}

func TestReader_readInt8To(t *testing.T) {
	var testCases = []struct {
		name    string
		expects []int8
	}{
		{name: "readInt8To", expects: []int8{1, 2, 3, 4}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeInt8(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				var got int8
				reader.readInt8To(&got)
				if got != expect {
					t.Errorf("readInt8To() = %v, want %v", got, expect)
				}
			}
		})
	}
}

func TestReader_readInt16To(t *testing.T) {
	var testCases = []struct {
		name    string
		expects []int16
	}{
		{name: "readInt16To", expects: []int16{1, 2, 3, 4}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeInt16(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				var got int16
				reader.readInt16To(&got)
				if got != expect {
					t.Errorf("readInt16To() = %v, want %v", got, expect)
				}
			}
		})
	}
}

func TestReader_readInt32To(t *testing.T) {
	var testCases = []struct {
		name    string
		expects []int32
	}{
		{name: "readInt32To", expects: []int32{1, 2, 3, 4}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeInt32(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				var got int32
				reader.readInt32To(&got)
				if got != expect {
					t.Errorf("readInt32To() = %v, want %v", got, expect)
				}
			}
		})
	}
}

func TestReader_readInt64To(t *testing.T) {
	var testCases = []struct {
		name    string
		expects []int64
	}{
		{name: "readInt64To", expects: []int64{1, 2, 3, 4}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeInt64(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				var got int64
				reader.readInt64To(&got)
				if got != expect {
					t.Errorf("readInt64To() = %v, want %v", got, expect)
				}
			}
		})
	}
}

func TestReader_readFloat32To(t *testing.T) {
	var testCases = []struct {
		name    string
		expects []float32
	}{
		{name: "readFloat32To", expects: []float32{1, 2, 3, 4}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeFloat32(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				var got float32
				reader.readFloat32To(&got)
				if got != expect {
					t.Errorf("readFloat32To() = %v, want %v", got, expect)
				}
			}
		})
	}
}

func TestReader_readFloat64To(t *testing.T) {
	var testCases = []struct {
		name    string
		expects []float64
	}{
		{name: "readFloat64To", expects: []float64{1, 2, 3, 4}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeFloat64(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				var got float64
				reader.readFloat64To(&got)
				if got != expect {
					t.Errorf("readFloat64To() = %v, want %v", got, expect)
				}
			}
		})
	}
}

func TestReader_readBoolTo(t *testing.T) {
	var testCases = []struct {
		name    string
		expects []bool
	}{
		{name: "readBoolTo", expects: []bool{true, false, true, false}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeBool(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				var got bool
				reader.readBoolTo(&got)
				if got != expect {
					t.Errorf("readBoolTo() = %v, want %v", got, expect)
				}
			}
		})
	}
}

func TestReader_readStringTo(t *testing.T) {
	var testCases = []struct {
		name    string
		expects []string
	}{
		{name: "readStringTo", expects: []string{"hello", "world", "foo", "bar"}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeString(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				var got string
				reader.readStringTo(&got)
				if got != expect {
					t.Errorf("readStringTo() = %v, want %v", got, expect)
				}
			}
		})
	}
}

func TestReader_readStringsTo(t *testing.T) {
	var testCases = []struct {
		name    string
		expects [][]string
	}{
		{name: "readStringsTo", expects: [][]string{
			{"hello"},
			{"hello", "world", "foo", "bar"},
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeStrings(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				var got []string
				reader.readStringsTo(&got)
				if len(got) != len(expect) {
					t.Errorf("readStringsTo() = %v, want %v", got, expect)
					continue
				}
				for i := range got {
					if got[i] != expect[i] {
						t.Errorf("readStringsTo() = %v, want %v", got, expect)
						break
					}
				}
			}
		})
	}
}

func TestReader_readUint8sTo(t *testing.T) {
	var testCases = []struct {
		name    string
		expects [][]uint8
	}{
		{name: "readUint8sTo", expects: [][]uint8{
			{1, 2, 3, 4},
			{5, 6, 7, 8},
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeUint8s(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				var got []uint8
				reader.readUint8sTo(&got)
				if len(got) != len(expect) {
					t.Errorf("readUint8sTo() = %v, want %v", got, expect)
					continue
				}
				for i := range got {
					if got[i] != expect[i] {
						t.Errorf("readUint8sTo() = %v, want %v", got, expect)
						break
					}
				}
			}
		})
	}
}

func TestReader_readUint16sTo(t *testing.T) {
	var testCases = []struct {
		name    string
		expects [][]uint16
	}{
		{name: "readUint16sTo", expects: [][]uint16{
			{1, 2, 3, 4},
			{5, 6, 7, 8},
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeUint16s(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				var got []uint16
				reader.readUint16sTo(&got)
				if len(got) != len(expect) {
					t.Errorf("readUint16sTo() = %v, want %v", got, expect)
					continue
				}
				for i := range got {
					if got[i] != expect[i] {
						t.Errorf("readUint16sTo() = %v, want %v", got, expect)
						break
					}
				}
			}
		})
	}
}

func TestReader_readUint32sTo(t *testing.T) {
	var testCases = []struct {
		name    string
		expects [][]uint32
	}{
		{name: "readUint32sTo", expects: [][]uint32{
			{1, 2, 3, 4},
			{5, 6, 7, 8},
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeUint32s(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				var got []uint32
				reader.readUint32sTo(&got)
				if len(got) != len(expect) {
					t.Errorf("readUint32sTo() = %v, want %v", got, expect)
					continue
				}
				for i := range got {
					if got[i] != expect[i] {
						t.Errorf("readUint32sTo() = %v, want %v", got, expect)
						break
					}
				}
			}
		})
	}
}

func TestReader_readUint64sTo(t *testing.T) {
	var testCases = []struct {
		name    string
		expects [][]uint64
	}{
		{name: "readUint64sTo", expects: [][]uint64{
			{1, 2, 3, 4},
			{5, 6, 7, 8},
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeUint64s(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				var got []uint64
				reader.readUint64sTo(&got)
				if len(got) != len(expect) {
					t.Errorf("readUint64sTo() = %v, want %v", got, expect)
					continue
				}
				for i := range got {
					if got[i] != expect[i] {
						t.Errorf("readUint64sTo() = %v, want %v", got, expect)
						break
					}
				}
			}
		})
	}
}

func TestReader_readInt8sTo(t *testing.T) {
	var testCases = []struct {
		name    string
		expects [][]int8
	}{
		{name: "readInt8sTo", expects: [][]int8{
			{1, 2, 3, 4},
			{5, 6, 7, 8},
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeInt8s(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				var got []int8
				reader.readInt8sTo(&got)
				if len(got) != len(expect) {
					t.Errorf("readInt8sTo() = %v, want %v", got, expect)
					continue
				}
				for i := range got {
					if got[i] != expect[i] {
						t.Errorf("readInt8sTo() = %v, want %v", got, expect)
						break
					}
				}
			}
		})
	}
}

func TestReader_readInt16sTo(t *testing.T) {
	var testCases = []struct {
		name    string
		expects [][]int16
	}{
		{name: "readInt16sTo", expects: [][]int16{
			{1, 2, 3, 4},
			{5, 6, 7, 8},
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeInt16s(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				var got []int16
				reader.readInt16sTo(&got)
				if len(got) != len(expect) {
					t.Errorf("readInt16sTo() = %v, want %v", got, expect)
					continue
				}
				for i := range got {
					if got[i] != expect[i] {
						t.Errorf("readInt16sTo() = %v, want %v", got, expect)
						break
					}
				}
			}
		})
	}
}

func TestReader_readInt32sTo(t *testing.T) {
	var testCases = []struct {
		name    string
		expects [][]int32
	}{
		{name: "readInt32sTo", expects: [][]int32{
			{1, 2, 3, 4},
			{5, 6, 7, 8},
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeInt32s(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				var got []int32
				reader.readInt32sTo(&got)
				if len(got) != len(expect) {
					t.Errorf("readInt32sTo() = %v, want %v", got, expect)
					continue
				}
				for i := range got {
					if got[i] != expect[i] {
						t.Errorf("readInt32sTo() = %v, want %v", got, expect)
						break
					}
				}
			}
		})
	}
}

func TestReader_readInt64sTo(t *testing.T) {
	var testCases = []struct {
		name    string
		expects [][]int64
	}{
		{name: "readInt64sTo", expects: [][]int64{
			{1, 2, 3, 4},
			{5, 6, 7, 8},
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeInt64s(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				var got []int64
				reader.readInt64sTo(&got)
				if len(got) != len(expect) {
					t.Errorf("readInt64sTo() = %v, want %v", got, expect)
					continue
				}
				for i := range got {
					if got[i] != expect[i] {
						t.Errorf("readInt64sTo() = %v, want %v", got, expect)
						break
					}
				}
			}
		})
	}
}

func TestReader_readFloat32sTo(t *testing.T) {
	var testCases = []struct {
		name    string
		expects [][]float32
	}{
		{name: "readFloat32sTo", expects: [][]float32{
			{1.0, 2.0, 3.0, 4.0},
			{5.0, 6.0, 7.0, 8.0},
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeFloat32s(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				var got []float32
				reader.readFloat32sTo(&got)
				if len(got) != len(expect) {
					t.Errorf("readFloat32sTo() = %v, want %v", got, expect)
					continue
				}
				for i := range got {
					if got[i] != expect[i] {
						t.Errorf("readFloat32sTo() = %v, want %v", got, expect)
						break
					}
				}
			}
		})
	}
}

func TestReader_readFloat64sTo(t *testing.T) {
	var testCases = []struct {
		name    string
		expects [][]float64
	}{
		{name: "readFloat64sTo", expects: [][]float64{
			{1.0, 2.0, 3.0, 4.0},
			{5.0, 6.0, 7.0, 8.0},
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeFloat64s(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				var got []float64
				reader.readFloat64sTo(&got)
				if len(got) != len(expect) {
					t.Errorf("readFloat64sTo() = %v, want %v", got, expect)
					continue
				}
				for i := range got {
					if got[i] != expect[i] {
						t.Errorf("readFloat64sTo() = %v, want %v", got, expect)
						break
					}
				}
			}
		})
	}
}

func TestReader_readBoolsTo(t *testing.T) {
	var testCases = []struct {
		name    string
		expects [][]bool
	}{
		{name: "readBoolsTo", expects: [][]bool{
			{true},
			{true, false, true, false},
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeBools(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				var got []bool
				reader.readBoolsTo(&got)
				if len(got) != len(expect) {
					t.Errorf("readBoolsTo() = %v, want %v", got, expect)
					continue
				}
				for i := range got {
					if got[i] != expect[i] {
						t.Errorf("readBoolsTo() = %v, want %v", got, expect)
						break
					}
				}
			}
		})
	}
}

func TestReader_readBytesTo(t *testing.T) {
	var testCases = []struct {
		name    string
		expects [][]byte
	}{
		{name: "readBytesTo", expects: [][]byte{
			{1, 2, 3, 4},
			{5, 6, 7, 8},
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			w := newWriter()
			for _, expect := range testCase.expects {
				w.writeBytes(expect)
			}
			buf := w.bytes()
			reader := newReader(buf)
			for _, expect := range testCase.expects {
				var got []byte
				reader.readBytesTo(&got)
				if len(got) != len(expect) {
					t.Errorf("readBytesTo() = %v, want %v", got, expect)
					continue
				}
				for i := range got {
					if got[i] != expect[i] {
						t.Errorf("readBytesTo() = %v, want %v", got, expect)
						break
					}
				}
			}
		})
	}
}
