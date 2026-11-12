package serialization

import "testing"

func BenchmarkReaderWriter_Struct(b *testing.B) {
	message := structFallbackPayload{ID: 42, Detail: structFallbackDetail{Value: -2}, Values: []uint16{3, 4}}
	data := []byte{0, 0, 0, 42, 255, 254, 0, 0, 0, 2, 0, 3, 0, 4}
	b.Run("Writer", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			w := GetWriter()
			w.Write(&message)
			if w.Err() != nil {
				b.Fatal(w.Err())
			}
			PutWriter(w)
		}
	})
	b.Run("ReaderReuse", func(b *testing.B) {
		b.ReportAllocs()
		var message structFallbackPayload
		for i := 0; i < b.N; i++ {
			r := GetReader(data)
			if err := r.Read(&message); err != nil {
				b.Fatal(err)
			}
			PutReader(r)
		}
	})
	b.Run("ReaderFresh", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var message structFallbackPayload
			r := GetReader(data)
			if err := r.Read(&message); err != nil {
				b.Fatal(err)
			}
			PutReader(r)
		}
	})
}

func BenchmarkReaderWriter_Integer(b *testing.B) {
	b.Run("WriteGeneric", func(b *testing.B) {
		var w Writer
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			w.buf = w.buf[:0]
			w.WriteInteger(uint32(42))
		}
		if w.Err() != nil {
			b.Fatal(w.Err())
		}
	})
	b.Run("WriteDynamic", func(b *testing.B) {
		var w Writer
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			w.buf = w.buf[:0]
			w.Write(uint32(42))
		}
		if w.Err() != nil {
			b.Fatal(w.Err())
		}
	})
	b.Run("ReadGeneric", func(b *testing.B) {
		r := Reader{buf: []byte{0, 0, 0, 42}}
		var value uint32
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			r.pos = 0
			if err := r.ReadInteger(&value); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("ReadDynamic", func(b *testing.B) {
		r := Reader{buf: []byte{0, 0, 0, 42}}
		var value uint32
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			r.pos = 0
			if err := r.Read(&value); err != nil {
				b.Fatal(err)
			}
		}
	})
}
