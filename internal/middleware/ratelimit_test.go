package middleware

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/pkg/iputil"
)

func TestRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_ = iputil.SetTrustedProxies(nil)
	now := time.Unix(1000, 0)
	r := gin.New()
	r.GET("/", rateLimit(2, time.Minute, 2, func() time.Time { return now }), func(c *gin.Context) { c.Status(204) })
	request := func(ip string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = ip + ":80"
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("8.8.8.%d", now.UnixNano()%255))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	for i := 0; i < 2; i++ {
		if w := request("192.0.2.1"); w.Code != 204 {
			t.Fatal(w.Code)
		}
	}
	w := request("192.0.2.1")
	if w.Code != 429 || w.Header().Get("Retry-After") != "60" || !strings.Contains(w.Body.String(), "\"error\":\"rate limit exceeded, please retry later\"") {
		t.Fatalf("bad response: %d %s %v", w.Code, w.Body, w.Header())
	}
	if w := request("192.0.2.2"); w.Code != 204 {
		t.Fatal("different peer blocked")
	}
	if w := request("192.0.2.3"); w.Code != 429 {
		t.Fatal("capacity must not evict live counters")
	}
	if w := request("192.0.2.1"); w.Code != 429 {
		t.Fatal("capacity bypass")
	}
	now = now.Add(time.Minute)
	if w := request("192.0.2.3"); w.Code != 204 {
		t.Fatal("expired entries not reclaimed")
	}
}

func TestRateLimitDisabled(t *testing.T) {
	r := gin.New()
	r.GET("/", RateLimit(0, time.Minute, 1), func(c *gin.Context) { c.Status(204) })
	for i := 0; i < 20; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
		if w.Code != 204 {
			t.Fatal(w.Code)
		}
	}
}
