package sharedlinks

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func makeContext(header, headerValue string, useTLS bool) *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	if header != "" {
		c.Request.Header.Set(header, headerValue)
	}
	if useTLS {
		c.Request.TLS = &tls.ConnectionState{}
	}
	return c
}

// TestSchemeFromContextXForwardedProto verifies that the X-Forwarded-Proto header
// is the primary signal (fixes the bug where scheme was always "https").
func TestSchemeFromContextXForwardedProto(t *testing.T) {
	tests := []struct {
		name       string
		headerVal  string
		useTLS     bool
		wantScheme string
	}{
		{
			name:       "X-Forwarded-Proto: https",
			headerVal:  "https",
			useTLS:     false,
			wantScheme: "https",
		},
		{
			name:       "X-Forwarded-Proto: http",
			headerVal:  "http",
			useTLS:     false,
			wantScheme: "http",
		},
		{
			name:       "X-Forwarded-Proto overrides TLS",
			headerVal:  "http",
			useTLS:     true,
			wantScheme: "http",
		},
		{
			name:       "X-Forwarded-Proto: https with TLS",
			headerVal:  "https",
			useTLS:     true,
			wantScheme: "https",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := makeContext("X-Forwarded-Proto", tt.headerVal, tt.useTLS)
			if got := schemeFromContext(c); got != tt.wantScheme {
				t.Fatalf("schemeFromContext() = %q, want %q", got, tt.wantScheme)
			}
		})
	}
}

// TestSchemeFromContextTLSFallback verifies that a TLS connection returns "https"
// when no X-Forwarded-Proto header is present.
func TestSchemeFromContextTLSFallback(t *testing.T) {
	c := makeContext("", "", true)
	if got := schemeFromContext(c); got != "https" {
		t.Fatalf("expected https for TLS request without header, got %q", got)
	}
}

// TestSchemeFromContextDefaultsToHTTP verifies the plain HTTP fallback.
func TestSchemeFromContextDefaultsToHTTP(t *testing.T) {
	c := makeContext("", "", false)
	if got := schemeFromContext(c); got != "http" {
		t.Fatalf("expected http for plain request without header, got %q", got)
	}
}
