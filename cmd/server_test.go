package cmd

import (
	"net/http"
	"testing"
	"time"
)

func TestNewHTTPServer(t *testing.T) {
	handler := http.NewServeMux()
	server := newHTTPServer("127.0.0.1:8080", handler)
	if server.Addr != "127.0.0.1:8080" || server.Handler != handler {
		t.Fatal("server must preserve its address and handler")
	}
	for _, tt := range []struct {
		name      string
		got, want time.Duration
	}{
		{"ReadHeaderTimeout", server.ReadHeaderTimeout, 5 * time.Second},
		{"ReadTimeout", server.ReadTimeout, 15 * time.Second},
		{"WriteTimeout", server.WriteTimeout, 45 * time.Second},
		{"IdleTimeout", server.IdleTimeout, 60 * time.Second},
	} {
		if tt.got != tt.want {
			t.Errorf("%s = %s, want %s", tt.name, tt.got, tt.want)
		}
	}
}
