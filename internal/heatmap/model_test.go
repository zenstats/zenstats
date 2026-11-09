package heatmap

import (
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestClickValidation(t *testing.T) {
	now := time.Now()
	valid := Click{ID: uuid.New(), Timestamp: now, Page: Page{Hostname: "example.test", Path: "/pricing", Version: "v1", Variant: "default", ViewportWidth: 1280, ViewportHeight: 720, DocumentWidth: 1280, DocumentHeight: 2000}, X: 400, Y: 600, RelativeX: .5, RelativeY: .5, Position: "document"}
	if err := valid.Validate(now); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*Click){
		"outside document":    func(c *Click) { c.Y = 2000 },
		"invalid coordinates": func(c *Click) { c.X = math.NaN() },
		"old timestamp":       func(c *Click) { c.Timestamp = now.Add(-2 * time.Hour) },
		"unbounded selector":  func(c *Click) { c.Element = "button[data-email=private@example.test]" },
		"query in path":       func(c *Click) { c.Page.Path = "/pricing?token=secret" },
	} {
		t.Run(name, func(t *testing.T) {
			c := valid
			change(&c)
			if c.Validate(now) == nil {
				t.Fatal("invalid click accepted")
			}
		})
	}
}

func TestCanonicalHashRoute(t *testing.T) {
	got, err := CanonicalPath("https://example.test/app?token=secret#/pricing?email=secret", true)
	if err != nil || got != "/app#/pricing" {
		t.Fatalf("%q %v", got, err)
	}
}

func TestHourBucketTimezoneBoundaries(t *testing.T) {
	for _, tc := range []struct{ zone, at, want string }{
		{"Asia/Kolkata", "2026-11-09T04:45:00Z", "2026-11-09T04:30:00Z"},
		{"Asia/Kathmandu", "2026-11-09T04:45:00Z", "2026-11-09T04:15:00Z"},
		{"America/New_York", "2026-11-01T05:40:00Z", "2026-11-01T05:00:00Z"},
		{"America/New_York", "2026-11-01T06:40:00Z", "2026-11-01T06:00:00Z"},
	} {
		t.Run(tc.zone+tc.at, func(t *testing.T) {
			at, _ := time.Parse(time.RFC3339, tc.at)
			want, _ := time.Parse(time.RFC3339, tc.want)
			cfg := DefaultConfig()
			cfg.Timezone = tc.zone
			r := NewRow(1, uuid.New(), Click{Timestamp: at}, cfg)
			if !r.Bucket.Equal(want) {
				t.Fatalf("got %s, want %s", r.Bucket, want)
			}
		})
	}
}

func TestPageRulesAndHosts(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.Paths = []string{"/blog/*", "/#/pricing", "/literal.+"}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"/blog/one", "/#/pricing", "/literal.+"} {
		if !cfg.Allows(p) {
			t.Fatal(p)
		}
	}
	for _, p := range []string{"/blog/one/two", "/pricing", "/literalXX"} {
		if cfg.Allows(p) {
			t.Fatal(p)
		}
	}
	for _, p := range []string{"/blog/[a-z]", "/pricing?email=secret", `/a\b`} {
		cfg.Paths = []string{p}
		if cfg.Validate() == nil {
			t.Fatal(p)
		}
	}
	if !HostAllowed("shop.example.test", "example.test", "https://*.example.test") || HostAllowed("example.test.evil.test", "example.test", "*.example.test") {
		t.Fatal("hostname boundaries")
	}
}
