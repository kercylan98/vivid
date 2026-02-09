package utils

import (
	"net"
	"regexp"
	"strconv"
	"strings"
)

var pathRegexp = regexp.MustCompile(`^/(?:[A-Za-z0-9\-._~!$&'()*+,;=:@]|%[0-9A-Fa-f]{2}|/)*$`)

// NormalizeAddress 规范化并校验 address。
func NormalizeAddress(address string) (string, bool) {
	address = strings.TrimSpace(address)
	if address == "" {
		return "", false
	}

	// 含端口时使用 SplitHostPort 校验 host:port 或 [ipv6]:port
	if strings.Contains(address, ":") {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return "", false
		}
		if !IsValidHost(host) {
			return "", false
		}
		if !IsValidPort(port) {
			return "", false
		}
		return address, true
	}

	// 无端口时只接受域名（不接受裸 IP）
	if net.ParseIP(address) != nil {
		return "", false
	}
	if !IsDomainName(address) {
		return "", false
	}
	return address, true
}

// NormalizeAddresses 规范化并校验 addresses，返回规范化后的地址列表。
func NormalizeAddresses(addresses []string) []string {
	if len(addresses) == 0 {
		return nil
	}
	var out []string
	for _, address := range addresses {
		if normalized, ok := NormalizeAddress(address); ok {
			out = append(out, normalized)
		}
	}
	return out
}

// NormalizePath 规范化并校验 path，要求以 / 开头。
func NormalizePath(path string) (string, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", false
	}
	if path[0] != '/' {
		return "", false
	}
	if !IsValidPath(path) {
		return "", false
	}
	return path, true
}

// FormatRefString 将 address/path 格式化为字符串。
// 当 address 含端口时，输出为 "host:port:/path"。
func FormatRefString(address, path string) string {
	if strings.Contains(address, ":") {
		return buildRefString(address, ":"+path)
	}
	return buildRefString(address, path)
}

// JoinPath 高效拼接路径片段，保留单个分隔符。
func JoinPath(base, segment string) string {
	if base == "" {
		return "/" + strings.TrimLeft(segment, "/")
	}
	segment = strings.TrimLeft(segment, "/")
	if base == "/" {
		return "/" + segment
	}
	if base[len(base)-1] == '/' {
		return base + segment
	}
	return base + "/" + segment
}

// IsValidHost 判断 host 是否为合法 IP 或域名。
func IsValidHost(host string) bool {
	if host == "" {
		return false
	}
	if net.ParseIP(host) != nil {
		return true
	}
	return IsDomainName(host)
}

// IsValidPort 判断端口是否合法。
func IsValidPort(port string) bool {
	if port == "" {
		return false
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		return false
	}
	return n >= 1 && n <= 65535
}

// IsValidPath 使用 URL path 允许字符，并额外支持 '@'。
func IsValidPath(path string) bool {
	if path == "" || path[0] != '/' {
		return false
	}
	return pathRegexp.MatchString(path)
}

func buildRefString(address, path string) string {
	var b strings.Builder
	b.Grow(len(address) + len(path))
	b.WriteString(address)
	b.WriteString(path)
	return b.String()
}
