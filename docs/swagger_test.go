package docs

import (
	"encoding/json"
	"os"
	"testing"
)

func TestSwaggerDocumentsRegisteredCriticalRoutes(t *testing.T) {
	content, err := os.ReadFile("swagger.json")
	if err != nil {
		t.Fatalf("read swagger.json: %v", err)
	}

	var spec struct {
		Paths map[string]map[string]any `json:"paths"`
	}
	if err := json.Unmarshal(content, &spec); err != nil {
		t.Fatalf("parse swagger.json: %v", err)
	}

	want := map[string][]string{
		"/stats/{domain}/aggregate":               {"get"},
		"/stats/{domain}/main-graph":              {"get"},
		"/stats/{domain}/breakdown":               {"get"},
		"/stats/{domain}/current-visitors":        {"get"},
		"/stats/{domain}/time_series":             {"get"},
		"/sites/{domain}":                         {"get", "put", "delete"},
		"/sites/{domain}/shield/country/{ruleId}": {"delete"},
	}

	for path, methods := range want {
		operations, ok := spec.Paths[path]
		if !ok {
			t.Fatalf("swagger path %q is missing", path)
		}
		for _, method := range methods {
			if _, ok := operations[method]; !ok {
				t.Fatalf("swagger operation %s %s is missing", method, path)
			}
		}
	}

	if _, exists := spec.Paths["/sites/{id}/shield/country/{ruleId}"]; exists {
		t.Fatal("swagger still contains stale country shield route /sites/{id}/shield/country/{ruleId}")
	}
	if _, exists := spec.Paths["/sites/:domain"]; exists {
		t.Fatal("swagger still contains gin-style route /sites/:domain")
	}
}
