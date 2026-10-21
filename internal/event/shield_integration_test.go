package event

import (
	"context"
	"fmt"
	"net"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/config"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/pkg/globals"
)

func TestShieldRuleChangesIntegration(t *testing.T) {
	if testing.Short() || os.Getenv("APP_ENV") != "test" {
		t.Skip("requires APP_ENV=test and PostgreSQL")
	}
	t.Chdir(t.TempDir()) // config.Load writes resolved configuration to disk.
	if err := config.Load("test"); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	db := postgresql.NewClient()
	defer db.Client.Close()
	if err := db.Client.Schema.Create(ctx); err != nil {
		t.Fatal(err)
	}
	previous := globals.GetDB()
	globals.SetDB(db)
	defer globals.SetDB(previous)
	site, err := db.Client.Site.Create().SetDomain(fmt.Sprintf("shield-%d.example.test", time.Now().UnixNano())).Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Client.Site.DeleteOne(site).Exec(ctx)
	other, err := db.Client.Site.Create().SetDomain(fmt.Sprintf("other-%d.example.test", time.Now().UnixNano())).Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Client.Site.DeleteOne(other).Exec(ctx)
	svc := service.GetSiteService()
	worker := &EventWork{siteService: svc, shutdownCtx: ctx}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/", nil)

	// Warm empty caches, mutate through the real CRUD methods, then check the
	// actual ingestion filters immediately. No cache expiry or mocks are involved.
	if worker.dropShieldRuleReferrer(site, "spam.invalid") {
		t.Fatal("unexpected initial referrer block")
	}
	deny, err := svc.AddShieldRuleReferrer(ctx, site.ID, "https://SPAM.INVALID.:443/path", "deny", "")
	if err != nil {
		t.Fatal(err)
	}
	if deny.Hostname != "spam.invalid" || !worker.dropShieldRuleReferrer(site, "sub.spam.invalid") {
		t.Fatal("referrer deny did not take effect immediately")
	}
	allow, err := svc.AddShieldRuleReferrer(ctx, site.ID, "trusted.spam.invalid", "allow", "")
	if err != nil {
		t.Fatal(err)
	}
	if worker.dropShieldRuleReferrer(site, "trusted.spam.invalid") {
		t.Fatal("specific allow did not override deny")
	}
	if worker.dropShieldRuleReferrer(other, "spam.invalid") {
		t.Fatal("rules leaked across sites")
	}
	if err := svc.RemoveShieldRuleReferrer(ctx, site.ID, allow.ID); err != nil {
		t.Fatal(err)
	}
	if !worker.dropShieldRuleReferrer(site, "trusted.spam.invalid") {
		t.Fatal("allow deletion left stale cache")
	}
	if err := svc.RemoveShieldRuleReferrer(ctx, site.ID, deny.ID); err != nil {
		t.Fatal(err)
	}
	if worker.dropShieldRuleReferrer(site, "spam.invalid") {
		t.Fatal("deny deletion left stale cache")
	}

	ip := net.ParseIP("203.0.113.15")
	if worker.dropShieldRuleIP(site, ip) {
		t.Fatal("unexpected initial IP block")
	}
	ipRule, err := svc.AddShieldRuleIP(c, site.ID, ip.String(), "deny", "")
	if err != nil {
		t.Fatal(err)
	}
	if !worker.dropShieldRuleIP(site, ip) {
		t.Fatal("IP deny did not take effect immediately")
	}
	if err := svc.RemoveShieldRuleIP(ctx, site.ID, ipRule.ID); err != nil {
		t.Fatal(err)
	}
	if worker.dropShieldRuleIP(site, ip) {
		t.Fatal("IP deletion left stale cache")
	}

	if worker.dropShieldRuleHostname(site, "other.example") {
		t.Fatal("unexpected initial hostname block")
	}
	hostRule, err := svc.AddShieldRuleHostname(ctx, site.ID, "ALLOWED.EXAMPLE.:443", "", "allow")
	if err != nil {
		t.Fatal(err)
	}
	if worker.dropShieldRuleHostname(site, "allowed.example") || !worker.dropShieldRuleHostname(site, "other.example") {
		t.Fatal("hostname allow list did not take effect")
	}
	if err := svc.RemoveShieldRuleHostname(c, site.ID, hostRule.ID); err != nil {
		t.Fatal(err)
	}
	if worker.dropShieldRuleHostname(site, "other.example") {
		t.Fatal("hostname deletion left stale cache")
	}
}
