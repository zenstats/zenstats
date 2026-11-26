package sharedlinks

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/config"
	"github.com/zenstats/zenstats/internal/auth"
	"github.com/zenstats/zenstats/internal/middleware"
	"github.com/zenstats/zenstats/internal/service"
	"golang.org/x/crypto/bcrypt"
	"time"
)

func TestAuthenticate(t *testing.T) {
	old := config.Conf.SecretKey
	config.Conf.SecretKey = "shared-test-key-01234567890123456789"
	t.Cleanup(func() { config.Conf.SecretKey = old })
	hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	link := &service.SharedLink{Slug: "test", SiteID: 42, PasswordHash: string(hash)}
	r := gin.New()
	r.POST("/api/share/:slug/authenticate", middleware.RateLimit(15, time.Minute, 100), authenticate(func(_ context.Context, slug string) (*service.SharedLink, error) {
		if slug != "test" {
			return nil, errors.New("not found")
		}
		return link, nil
	}))
	for _, tc := range []struct {
		slug, body string
		want       int
	}{
		{"missing", `{"password":"password"}`, 404},
		{"test", `{}`, 401},
		{"test", `{"password":"wrong"}`, 401},
		{"test", `{"password":"password"}`, 200},
	} {
		req := httptest.NewRequest("POST", "/api/share/"+tc.slug+"/authenticate", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Fatalf("got %d: %s", w.Code, w.Body)
		}
		if tc.want == 200 {
			var body struct {
				Data struct {
					Token     string `json:"token"`
					ExpiresIn int    `json:"expires_in"`
				}
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			claims, err := auth.ParseSharedLinkToken(body.Data.Token)
			if err != nil || !claims.Matches("test", 42, string(hash)) || body.Data.ExpiresIn != 3600 {
				t.Fatalf("bad token response: %+v %v", body, err)
			}
		}
	}
	link.PasswordHash = ""
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/share/test/authenticate", strings.NewReader("{}")))
	if w.Code != 200 {
		t.Fatalf("unprotected: %s", w.Body)
	}
	for i := 0; i < 20; i++ {
		w = httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/api/share/test/authenticate", strings.NewReader("{}")))
	}
	if w.Code != 429 || w.Header().Get("Retry-After") == "" {
		t.Fatalf("auth not rate limited: %d", w.Code)
	}
}

func TestSharedLinkURL(t *testing.T) {
	old := config.Conf.BaseURL
	t.Cleanup(func() { config.Conf.BaseURL = old })
	link := &service.SharedLink{Slug: "slug"}
	config.Conf.BaseURL = "https://stats.example/prefix///"
	if got := toResponse(link, "http", "attacker.example").URL; got != "https://stats.example/prefix/share/slug" {
		t.Fatal(got)
	}
	config.Conf.BaseURL = ""
	if got := toResponse(link, "http", "localhost:8080").URL; got != "http://localhost:8080/share/slug" {
		t.Fatal(got)
	}
}
