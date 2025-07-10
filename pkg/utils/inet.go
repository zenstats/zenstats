package utils

import (
	"fmt"
	"net"

	"github.com/zenstats/zenstats/internal/store/postgresql/ent/schema"
)

func ParseInet(ip string) (*schema.Inet, error) {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return nil, fmt.Errorf("invalid IP address: %s", ip)
	}
	return &schema.Inet{IP: parsed}, nil
}
