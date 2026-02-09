package cluster

import (
	"net"
	"strings"
)

// AllowJoinByDC 判断给定 DC 是否在允许加入的 DC 列表中；allowDCs 为空表示不限制。
func AllowJoinByDC(dc string, allowDCs []string) bool {
	if len(allowDCs) == 0 {
		return true
	}
	if dc == "" {
		dc = "_default"
	}
	for _, d := range allowDCs {
		if d == "" {
			d = "_default"
		}
		if d == dc {
			return true
		}
	}
	return false
}

// AllowJoinByAddress 判断给定地址（host 或 host:port）是否在允许列表中；allowList 可为精确 host、host:port 或 CIDR；空表示不限制。
func AllowJoinByAddress(addr string, allowList []string) bool {
	if len(allowList) == 0 {
		return true
	}
	host := addr
	if idx := strings.LastIndex(addr, ":"); idx >= 0 {
		host = addr[:idx]
	}
	ip := net.ParseIP(host)
	for _, a := range allowList {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		if strings.Contains(a, "/") {
			_, cidr, err := net.ParseCIDR(a)
			if err != nil {
				continue
			}
			if ip != nil && cidr.Contains(ip) {
				return true
			}
		} else if a == host || a == addr {
			return true
		} else if ip != nil && net.ParseIP(a) != nil && a == host {
			return true
		}
	}
	return false
}
