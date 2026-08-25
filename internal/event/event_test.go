package event

import (
	"net"
	"net/url"
	"testing"

	"github.com/zenstats/zenstats/internal/store/clickhouse/models"
)

func TestSanitizeStoredURLStripsQueryAndFragment(t *testing.T) {
	cases := []struct {
		raw      string
		hashMode bool
		want     string
	}{
		{"https://example.com/path?utm_source=x&token=secret#frag", false, "https://example.com/path"},
		{"https://example.com/path?utm_source=x", false, "https://example.com/path"},
		{"https://example.com/", false, "https://example.com/"},
		{"https://user:pw@example.com/p?q=1", false, "https://example.com/p"},
		// hash-routing keeps the fragment path but drops any query inside it
		{"https://example.com/#/dashboard?token=abc", true, "https://example.com/#/dashboard"},
		{"https://example.com/app?x=1#/settings", true, "https://example.com/app#/settings"},
		{"https://example.com/#/dashboard", false, "https://example.com/"},
	}
	for _, c := range cases {
		parsed, err := url.Parse(c.raw)
		if err != nil {
			t.Fatalf("parse %q: %v", c.raw, err)
		}
		if got := sanitizeStoredURL(c.raw, parsed, c.hashMode); got != c.want {
			t.Errorf("sanitizeStoredURL(%q, hash=%v) = %q, want %q", c.raw, c.hashMode, got, c.want)
		}
	}
}

func TestSanitizeStoredURLFallbackWithoutParsedURL(t *testing.T) {
	if got := sanitizeStoredURL("http://exa mple.com/p?a=1#x", nil, false); got != "http://exa mple.com/p" {
		t.Fatalf("fallback truncation failed, got %q", got)
	}
}

func TestSplitIPColumns(t *testing.T) {
	v4 := &models.Events{IP: net.ParseIP("203.0.113.9")}
	splitIPColumns(v4)
	if len(v4.IP) != net.IPv4len || v4.IP.String() != "203.0.113.9" || v4.IPv6 != nil {
		t.Fatalf("IPv4: got ip=%v ipv6=%v", v4.IP, v4.IPv6)
	}

	mapped := &models.Events{IP: net.ParseIP("::ffff:198.51.100.7")}
	splitIPColumns(mapped)
	if len(mapped.IP) != net.IPv4len || mapped.IP.String() != "198.51.100.7" || mapped.IPv6 != nil {
		t.Fatalf("IPv4-mapped: got ip=%v ipv6=%v", mapped.IP, mapped.IPv6)
	}

	v6 := &models.Events{IP: net.ParseIP("2001:db8::1")}
	splitIPColumns(v6)
	if v6.IP != nil || v6.IPv6 == nil || v6.IPv6.String() != "2001:db8::1" {
		t.Fatalf("IPv6: got ip=%v ipv6=%v", v6.IP, v6.IPv6)
	}

	empty := &models.Events{}
	splitIPColumns(empty)
	if empty.IP != nil || empty.IPv6 != nil {
		t.Fatalf("nil IP should stay nil, got ip=%v ipv6=%v", empty.IP, empty.IPv6)
	}
}
