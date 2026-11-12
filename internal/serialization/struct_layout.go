package serialization

import (
	"reflect"
	"sync"
)

var structLayouts sync.Map

// structLayout 仅缓存类型元数据，不持有消息实例或可变的 Codec 配置。
type structLayout struct {
	custom bool
	fields []int
}

func loadStructLayout(typ reflect.Type) *structLayout {
	if layout, ok := structLayouts.Load(typ); ok {
		return layout.(*structLayout)
	}
	pointerType := reflect.PointerTo(typ)
	layout := &structLayout{custom: pointerType.Implements(messageCodecType)}
	for i := 0; i < typ.NumField(); i++ {
		if typ.Field(i).IsExported() {
			layout.fields = append(layout.fields, i)
		}
	}
	actual, _ := structLayouts.LoadOrStore(typ, layout)
	return actual.(*structLayout)
}
