package heatmap

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestScrollValidation(t *testing.T) {
	now := time.Now()
	r := ScrollReport{ID: uuid.New(), PageViewID: uuid.New(), StartedAt: now, Timestamp: now, Revision: 1,
		Page: Page{Hostname: "example.test", Path: "/", Version: "v1", Variant: "default", ViewportWidth: 1280, ViewportHeight: 720, DocumentWidth: 1280, DocumentHeight: 2000}, MaxBottom: 720, MaxHeight: 2000}
	if err := r.Validate(now); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*ScrollReport){
		"missing visit":      func(r *ScrollReport) { r.PageViewID = uuid.Nil },
		"before start":       func(r *ScrollReport) { r.Timestamp = now.Add(-time.Second) },
		"old visit":          func(r *ScrollReport) { r.StartedAt = now.Add(-49 * time.Hour) },
		"shrunk denominator": func(r *ScrollReport) { r.MaxHeight = 1900 },
		"outside document":   func(r *ScrollReport) { r.MaxBottom = 2001 },
		"report limit":       func(r *ScrollReport) { r.Revision = 13 },
		"query value":        func(r *ScrollReport) { r.Page.Path = "/?token=secret" },
	} {
		t.Run(name, func(t *testing.T) {
			bad := r
			mutate(&bad)
			if bad.Validate(now) == nil {
				t.Fatal("invalid report accepted")
			}
		})
	}
	if DefaultConfig().ScrollEnabled {
		t.Fatal("scroll enabled by default")
	}
}
