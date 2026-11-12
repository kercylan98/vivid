package serialization

import (
	"reflect"
	"sync"
)

var structLayouts sync.Map

// structLayout 仅缓存类型元数据，不持有消息实例或可变的 Codec 配置。
type structLayout struct {
	custom       bool
	fields       []int
	readerFields []readerField
}

func loadStructLayout(typ reflect.Type) *structLayout {
	if layout, ok := structLayouts.Load(typ); ok {
		return layout.(*structLayout)
	}
	pointerType := reflect.PointerTo(typ)
	layout := &structLayout{custom: pointerType.Implements(messageCodecType)}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.IsExported() {
			layout.fields = append(layout.fields, i)
			reader := readerField{offset: field.Offset, kind: field.Type.Kind(), typ: field.Type}
			if reader.kind == reflect.Struct && field.Type != timeType {
				reader.layout = loadStructLayout(field.Type)
			}
			if reader.kind == reflect.Slice {
				reader.slice = loadSliceReader(field.Type.Elem().Kind())
			}
			layout.readerFields = append(layout.readerFields, reader)
		}
	}
	actual, _ := structLayouts.LoadOrStore(typ, layout)
	return actual.(*structLayout)
}
