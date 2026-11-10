package iputil

import (
	"net/http/httptest"
	"testing"
)

func TestClientIP(t *testing.T) {
	for _, tc := range []struct{ name, forwarded, real, remote, want string }{
		{"forwarded", "8.8.8.8, 10.0.0.1", "1.1.1.1", "127.0.0.1:80", "8.8.8.8"},
		{"invalid header falls back", "unknown", "8.8.8.8", "127.0.0.1:80", "8.8.8.8"},
		{"address with port", "8.8.8.8:1234", "", "127.0.0.1:80", "8.8.8.8"},
		{"ipv6 port", "[2001:4860:4860::8888]:1234", "", "127.0.0.1:80", "2001:4860:4860::8888"},
		{"bare ipv6 peer", "", "", "2001:4860:4860::8888", "2001:4860:4860::8888"},
		{"invalid chain is not a visitor", "garbage, 1.1.1.1", "invalid", "8.8.8.8:80", "8.8.8.8"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/", nil)
			r.Header.Set("X-Forwarded-For", tc.forwarded)
			r.Header.Set("X-Real-IP", tc.real)
			r.RemoteAddr = tc.remote
			if got := ClientIP(r); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}
