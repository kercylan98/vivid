package utils

import (
	"net"
	"strings"
)

// NormalizeRemoteAddress 校验供其他节点拨号的 host:port，裁剪首尾空白，不解析 DNS。
// 通配主机仅用于监听，端口必须为 1–65535 的十进制数字。
func NormalizeRemoteAddress(address string) (string, bool) {
	address = strings.TrimSpace(address)
	host, port, err := net.SplitHostPort(address)
	if err != nil || !IsValidPort(port) || port[0] == '+' {
		return "", false
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsUnspecified() {
			return "", false
		}
	} else if !IsDomainName(host) {
		return "", false
	}
	return address, true
}
