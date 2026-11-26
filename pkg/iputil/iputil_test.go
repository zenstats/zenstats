package iputil

import (
	"net"
	"net/http/httptest"
	"testing"
)

func TestClientIP(t *testing.T) {
	if err := SetTrustedProxies([]string{"127.0.0.1", "::1", "10.0.0.0/8", "172.16.0.0/12", "fc00::/7"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = SetTrustedProxies(nil) })
	for _, tc := range []struct{ name, forwarded, real, remote, want string }{
		{"direct forged headers", "1.1.1.1", "8.8.8.8", "203.0.113.2:456", "203.0.113.2"},
		{"single proxy", "8.8.8.8", "", "127.0.0.1:80", "8.8.8.8"},
		{"multiple trusted hops", "8.8.8.8, 10.0.0.1, 172.16.0.1", "", "127.0.0.1:80", "8.8.8.8"},
		{"injected leftmost", "1.1.1.1, 203.0.113.5, 10.0.0.1", "", "127.0.0.1:80", "203.0.113.5"},
		{"untrusted intermediate", "8.8.8.8, 203.0.113.6", "", "127.0.0.1:80", "203.0.113.6"},
		{"invalid hop skipped", "garbage, 8.8.8.8, bad, 10.0.0.1", "", "127.0.0.1:80", "8.8.8.8"},
		{"all proxies real fallback", "10.0.0.1, 172.16.0.1", "8.8.8.8", "127.0.0.1:80", "8.8.8.8"},
		{"invalid headers peer fallback", "unknown", "invalid", "127.0.0.1:80", "127.0.0.1"},
		{"header ports rejected", "8.8.8.8:1234", "", "127.0.0.1:80", "127.0.0.1"},
		{"IPv6 trusted", "2001:4860:4860::8888, fc00::123", "", "[::1]:80", "2001:4860:4860::8888"},
		{"bare IPv6 peer", "1.1.1.1", "", "2001:4860:4860::8888", "2001:4860:4860::8888"},
		{"invalid peer fail closed", "8.8.8.8", "1.1.1.1", "garbage", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/", nil)
			r.RemoteAddr = tc.remote
			r.Header.Set("X-Forwarded-For", tc.forwarded)
			r.Header.Set("X-Real-IP", tc.real)
			if got := ClientIP(r); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
	// Malformed updates are atomic; clearing explicitly disables all forwarding trust.
	if err := SetTrustedProxies([]string{"10.0.0.0/8", "not-an-ip"}); err == nil {
		t.Fatal("invalid proxy accepted")
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:80"
	r.Header.Set("X-Forwarded-For", "8.8.8.8")
	if got := ClientIP(r); got != "8.8.8.8" {
		t.Fatalf("failed update changed list: %s", got)
	}
	if err := SetTrustedProxies([]string{}); err != nil {
		t.Fatal(err)
	}
	if got := ClientIP(r); got != "127.0.0.1" {
		t.Fatalf("clear failed: %s", got)
	}
}

func TestIsLocal(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "10.255.255.255", "172.16.0.1", "172.31.255.255", "192.168.1.1", "169.254.1.1", "224.0.0.1", "0.0.0.0", "0.1.2.3", "100.64.0.0", "100.127.255.255", "::", "::1", "fc00::1", "fdff::1", "fe80::1", "ff02::1", "::ffff:192.168.1.1", "::ffff:100.64.0.1"} {
		if !IsLocal(net.ParseIP(ip)) {
			t.Errorf("%s should be local", ip)
		}
	}
	for _, ip := range []string{"8.8.8.8", "172.15.255.255", "172.32.0.0", "100.63.255.255", "100.128.0.0", "2001:4860:4860::8888", "bad"} {
		if IsLocal(net.ParseIP(ip)) {
			t.Errorf("%s should not be local", ip)
		}
	}
}
