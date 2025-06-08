package metrics

// Tag 指标标签类型。
// 用于为指标添加多维度的元数据，便于分类和查询。
type Tag struct {
	Key   string
	Value string
}

// WithTag 创建新的标签
func WithTag(key, value string) Tag {
	return Tag{Key: key, Value: value}
}

// WithTagsFromMap 从 map 创建标签切片。
// 这是一个便利函数，用于将 map[string]string 转换为 Tag 切片。
func WithTagsFromMap(m map[string]string) []Tag {
	tags := make([]Tag, 0, len(m))
	for k, v := range m {
		tags = append(tags, WithTag(k, v))
	}
	return tags
}
