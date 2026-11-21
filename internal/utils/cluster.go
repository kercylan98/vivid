package utils

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// IsValidClusterLabel 校验集群名称或机房标识，不裁剪或规范化调用方输入。
func IsValidClusterLabel(label string) bool {
	return len(label) > 0 && len(label) <= 128 && utf8.ValidString(label) &&
		!strings.ContainsFunc(label, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) })
}
