package event

import (
	"testing"

	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	"github.com/zenstats/zenstats/pkg/utils"
)

func TestReferrerRulesRespectSpecificityAndHostBoundaries(t *testing.T) {
	rules := []*ent.ShieldRulesReferrer{
		{Hostname: "Trusted.Spam.Example:443", Action: "allow"},
		{Hostname: "spam.example.", Action: "deny"},
	}
	for _, tc := range []struct{ host, action string }{
		{"https://SPAM.EXAMPLE.:443/path", "deny"}, {"sub.spam.example", "deny"},
		{"trusted.spam.example", "allow"}, {"notspam.example", ""}, {"spam.example.attacker.test", ""},
	} {
		if got := referrerRuleAction(rules, tc.host); got != tc.action {
			t.Errorf("%q action=%q want=%q", tc.host, got, tc.action)
		}
	}
	if got := utils.NormalizeHostname("https://EXAMPLE.com.:8443/path"); got != "example.com" {
		t.Fatalf("hostname=%q", got)
	}
}
