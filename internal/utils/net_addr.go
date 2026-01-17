package utils

import (
	"errors"
	"fmt"
	"net"
	"regexp"
)

var ErrUnknownNetwork = errors.New("unknown network")

var domainRegexp = regexp.MustCompile(`^(?i:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)(?:\.(?i:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?))*$`)

// ResolveNetAddr 解析网络地址
//
// 参数:
//   - network: 网络类型
//   - address: 地址
//
// 返回:
//   - net.Addr: 网络地址
//   - error: 错误
//
// 错误:
//   - ErrUnknownNetwork: 未知网络类型
//   - fmt.Errorf: 其他错误
//
// 示例:
//
//	addr, err := utils.ResolveNetAddr("tcp", "127.0.0.1:8080")
//	if err != nil {
//		log.Fatalf("Failed to resolve network address: %v", err)
//	}
//	fmt.Println(addr)
func ResolveNetAddr(network string, address string) (net.Addr, error) {
	switch network {
	case "udp":
		return net.ResolveUDPAddr(network, address)
	case "tcp":
		return net.ResolveTCPAddr(network, address)
	case "unix":
		return net.ResolveUnixAddr(network, address)
	case "ip":
		return net.ResolveIPAddr(network, address)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownNetwork, network)
	}
}

// IsAddrMissingPort 判断地址是否缺少端口。
func IsAddrMissingPort(address string) bool {
	if address == "" {
		return true
	}
	_, _, err := net.SplitHostPort(address)
	return err != nil
}

// IsDomainName 判断地址是否为有效域名。
func IsDomainName(address string) bool {
	if len(address) > 253 {
		return false
	}
	return domainRegexp.MatchString(address)
}
