package utils

import (
	"net/url"
	"strings"
)

// NormalizeHostname accepts a URL or host (optionally with a port), and returns
// a lowercase hostname without a trailing DNS dot. Invalid input returns empty.
func NormalizeHostname(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") && !strings.HasPrefix(raw, "//") {
		raw = "//" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimRight(parsed.Hostname(), "."))
}
