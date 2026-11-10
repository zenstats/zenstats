package performance

import (
	"github.com/google/uuid"
	"math"
	"testing"
	"time"
)

func TestReportValidation(t *testing.T) {
	now := time.Now()
	zero := float64(0)
	r := Report{ID: uuid.New(), LifecycleID: uuid.New(), Kind: "metric", Revision: 1, NavigationStartedAt: now, ReportedAt: now, Hostname: "example.test", Path: "/pricing", Release: "v1", NavigationType: "navigate", MetricID: "v6-test", MetricName: "CLS", Value: &zero}
	if err := r.Validate(now); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*Report){
		"unknown metric": func(r *Report) { r.MetricName = "FID" }, "nonfinite": func(r *Report) { v := math.Inf(1); r.Value = &v },
		"negative": func(r *Report) { v := -1.; r.Value = &v }, "missing value": func(r *Report) { r.Value = nil },
		"context with value": func(r *Report) { r.Kind = "context" }, "soft navigation": func(r *Report) { r.NavigationType = "soft-navigation" },
		"query leak": func(r *Report) { r.Path = "/pricing?email=private" }, "unbounded revisions": func(r *Report) { r.Revision = 33 },
		"before navigation": func(r *Report) { r.ReportedAt = now.Add(-time.Second) },
	} {
		t.Run(name, func(t *testing.T) {
			bad := r
			mutate(&bad)
			if bad.Validate(now) == nil {
				t.Fatal("invalid report accepted")
			}
		})
	}
	context := r
	context.Kind = "context"
	context.MetricID = ""
	context.MetricName = ""
	context.Value = nil
	context.Supported = []string{"CLS", "LCP"}
	if err := context.Validate(now); err != nil {
		t.Fatal(err)
	}
	if DefaultConfig().Enabled {
		t.Fatal("enabled by default")
	}
}
func TestMissingAndSmallSamples(t *testing.T) {
	missing := Metric{Name: "INP"}
	missing.Complete(100)
	if missing.P75 != nil || missing.Good != 0 || missing.Coverage != 0 || missing.Missing != 100 || missing.Rating != "insufficient" {
		t.Fatal(missing)
	}
	value := 0.
	cls := Metric{Name: "CLS", P75: &value, Samples: 100, Good: 100}
	cls.Complete(100)
	if cls.Rating != "good" || cls.P75 == nil || cls.Coverage != 100 {
		t.Fatal(cls)
	}
	cls.Samples = 99
	cls.Complete(100)
	if cls.Rating != "insufficient" {
		t.Fatal(cls)
	}
}
