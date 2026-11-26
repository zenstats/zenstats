package middleware

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/config"
	"github.com/zenstats/zenstats/internal/auth"
	"github.com/zenstats/zenstats/internal/service"
)

func TestSharedLinkAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	old := config.Conf.SecretKey
	config.Conf.SecretKey = "shared-test-key-01234567890123456789"
	t.Cleanup(func() { config.Conf.SecretKey = old })
	link := &service.SharedLink{Slug: "test", SiteID: 42, PasswordHash: "hash"}
	token, err := auth.GenerateSharedLinkToken(link.Slug, link.SiteID, link.PasswordHash, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	wrongSlug, _ := auth.GenerateSharedLinkToken("other", 42, "hash", time.Hour)
	wrongSite, _ := auth.GenerateSharedLinkToken("test", 43, "hash", time.Hour)
	access, _ := auth.GenerateToken(42)
	r := gin.New()
	r.GET("/", sharedLinkAuth(func(context.Context, string) (*service.SharedLink, error) { return link, nil }), func(c *gin.Context) {
		if c.GetInt64("shared_link_site_id") != 42 || c.GetString("auth_type") != "shared_link" {
			t.Error("missing shared identity")
		}
		c.Status(204)
	})
	for _, tc := range []struct {
		name, query, header, value string
		want                       int
	}{
		{"missing", "?slug=test", "", "", 401},
		{"query password", "?slug=test&link_password=password", "", "", 401},
		{"header password", "?slug=test", "X-Shared-Link-Password", "password", 401},
		{"valid", "?slug=test", "X-Shared-Link-Token", token, 204},
		{"wrong slug", "?slug=test", "X-Shared-Link-Token", wrongSlug, 401},
		{"wrong site", "?slug=test", "X-Shared-Link-Token", wrongSite, 401},
		{"access subject", "?slug=test", "X-Shared-Link-Token", access, 401},
		{"invalid", "?slug=test", "X-Shared-Link-Token", "invalid", 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/"+tc.query, nil)
			req.Header.Set(tc.header, tc.value)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("got %d: %s", w.Code, w.Body)
			}
		})
	}
	link.PasswordHash = "changed"
	req := httptest.NewRequest("GET", "/?slug=test", nil)
	req.Header.Set("X-Shared-Link-Token", token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatal("old token accepted")
	}
	link.PasswordHash = ""
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/?slug=test", nil))
	if w.Code != 204 {
		t.Fatal("public link rejected")
	}
}
