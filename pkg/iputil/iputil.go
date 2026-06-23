// Package iputil 提供 IP 地址相关的工具函数。
package iputil

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/zenstats/zenstats/internal/store/postgresql/ent/schema"
)

// ClientIP 从 HTTP 请求中提取客户端 IP（优先 X-Forwarded-For）。
func ClientIP(r *http.Request) string {
	xForwardedFor := r.Header.Get("X-Forwarded-For")
	ip := strings.TrimSpace(strings.Split(xForwardedFor, ",")[0])
	if ip != "" {
		return ip
	}

	ip = strings.TrimSpace(r.Header.Get("X-Real-Ip"))
	if ip != "" {
		return ip
	}

	if ip, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr)); err == nil {
		return ip
	}

	return ""
}

// IsLocal 检查 IP 是否为本地/内网地址。
func IsLocal(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() {
		return true
	}

	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}

	return ip4[0] == 10 ||
		(ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31) ||
		(ip4[0] == 169 && ip4[1] == 254) ||
		(ip4[0] == 192 && ip4[1] == 168)
}

// IsLocalAddr 检查 IP 字符串是否为本地/内网地址。
func IsLocalAddr(ip string) bool {
	return IsLocal(net.ParseIP(ip))
}

// ParseInet 将 IP 字符串解析为 Ent schema.Inet 类型。
func ParseInet(ip string) (*schema.Inet, error) {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return nil, fmt.Errorf("invalid IP address: %s", ip)
	}
	return &schema.Inet{IP: parsed}, nil
}
