// Package iputil 提供 IP 地址相关的工具函数。
package iputil

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/zenstats/zenstats/internal/store/postgresql/ent/schema"
)

var proxies struct {
	sync.RWMutex
	networks []*net.IPNet
}

// SetTrustedProxies atomically replaces the trusted proxy list; invalid input leaves it unchanged.
func SetTrustedProxies(cidrs []string) error {
	networks := make([]*net.IPNet, 0, len(cidrs))
	for _, value := range cidrs {
		value = strings.TrimSpace(value)
		if ip := net.ParseIP(value); ip != nil {
			bits := 128
			if v4 := ip.To4(); v4 != nil {
				ip = v4
				bits = 32
			}
			networks = append(networks, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
		} else {
			_, network, err := net.ParseCIDR(value)
			if err != nil {
				return fmt.Errorf("invalid trusted proxy %q: %w", value, err)
			}
			networks = append(networks, network)
		}
	}
	proxies.Lock()
	proxies.networks = networks
	proxies.Unlock()
	return nil
}

// ClientIP trusts forwarding headers only when the immediate peer is trusted.
func ClientIP(r *http.Request) string {
	peer := strings.TrimSpace(r.RemoteAddr)
	if host, _, err := net.SplitHostPort(peer); err == nil {
		peer = host
	}
	ip := net.ParseIP(peer)
	if ip == nil {
		return ""
	}
	proxies.RLock()
	defer proxies.RUnlock()
	trusted := func(ip net.IP) bool {
		for _, network := range proxies.networks {
			if network.Contains(ip) {
				return true
			}
		}
		return false
	}
	if !trusted(ip) {
		return ip.String()
	}
	hops := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		hop := net.ParseIP(strings.TrimSpace(hops[i]))
		if hop != nil && !trusted(hop) {
			return hop.String()
		}
	}
	if real := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); real != nil {
		return real.String()
	}
	return ip.String()
}

// IsLocal includes private, loopback, link-local, unspecified and shared-address space.
func IsLocal(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	v4 := ip.To4()
	return v4 != nil && (v4[0] == 0 || (v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127))
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
