package schema

import (
	"database/sql/driver"
	"fmt"
	"net"
)

// Inet 类型用于处理 PostgreSQL 的 inet 类型
type Inet net.IPNet

// Value 实现 driver.Valuer 接口
func (i Inet) Value() (driver.Value, error) {
	if i.IP == nil {
		return nil, nil
	}
	return i.String(), nil
}

// Scan 实现 sql.Scanner 接口
func (i *Inet) Scan(src interface{}) error {
	if src == nil {
		return nil
	}

	switch src := src.(type) {
	case string:
		_, ipnet, err := net.ParseCIDR(src)
		if err != nil {
			ip := net.ParseIP(src)
			if ip == nil {
				return fmt.Errorf("invalid IP address: %s", src)
			}
			ipnet = &net.IPNet{IP: ip, Mask: net.CIDRMask(32, 32)}
		}
		*i = Inet(*ipnet)
		return nil
	case []byte:
		return i.Scan(string(src))
	default:
		return fmt.Errorf("cannot scan %T into Inet", src)
	}
}

// String 返回 IP 或 CIDR 字符串表示
func (i Inet) String() string {
	ipnet := net.IPNet(i)
	if ipnet.Mask == nil {
		return ipnet.IP.String()
	}
	return ipnet.String()
}

// Contains 判断 IP 是否在 CIDR 中
func (i Inet) Contains(ip net.IP) bool {
	ipnet := net.IPNet(i)
	return ipnet.Contains(ip)
}
