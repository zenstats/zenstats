package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDoesNotWriteConfig(t *testing.T) {
	previous := Conf
	t.Cleanup(func() {
		mu.Lock()
		Conf = previous
		mu.Unlock()
	})
	t.Chdir(t.TempDir())
	t.Setenv("ZENSTATS_SECRET_KEY", "test-only-signing-key-with-at-least-32-bytes")
	t.Setenv("ZENSTATS_SMTP_PASSWORD", "test-only-sensitive-value")

	if err := Load("test"); err != nil {
		t.Fatal(err)
	}
	if Conf.SMTP.Password != "test-only-sensitive-value" {
		t.Fatal("environment overrides must still be loaded")
	}
	for _, path := range []string{filepath.Join("config", "config_test.yaml"), "config"} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("Load must not create %s; stat error: %v", path, err)
		}
	}
}

func TestLoadPreservesExternalConfig(t *testing.T) {
	previous := Conf
	t.Cleanup(func() {
		mu.Lock()
		Conf = previous
		mu.Unlock()
	})
	t.Chdir(t.TempDir())
	t.Setenv("ZENSTATS_SECRET_KEY", "test-only-signing-key-with-at-least-32-bytes")
	t.Setenv("ZENSTATS_BASE_URL", "")
	if err := os.Mkdir("config", 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("config", "config_test.yaml")
	contents := "base_url: https://override.example.test\n"
	if err := os.WriteFile(path, []byte(contents), 0400); err != nil {
		t.Fatal(err)
	}
	if err := Load("test"); err != nil {
		t.Fatal(err)
	}
	if Conf.BaseURL != "https://override.example.test" {
		t.Fatalf("external override not loaded: %q", Conf.BaseURL)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != contents {
		t.Fatalf("external config changed: contents=%q error=%v", got, err)
	}
}

func TestSecurityAndStatsConfig(t *testing.T) {
	previous := Conf
	t.Cleanup(func() { mu.Lock(); Conf = previous; mu.Unlock() })
	t.Chdir(t.TempDir())
	t.Setenv("ZENSTATS_SECRET_KEY", "test-only-signing-key-with-at-least-32-bytes")
	for _, key := range []string{"TRUSTED_PROXIES", "RATE_LIMIT_AUTH_PER_MINUTE", "CLICKHOUSE_EVENTS_TTL_DAYS", "CLICKHOUSE_SESSIONS_TTL_DAYS", "STATS_MAX_CONCURRENT", "STATS_MAX_THREADS", "STATS_MAX_MEMORY_USAGE", "STATS_MAX_EXECUTION_TIME", "STATS_QUERY_TIMEOUT_SECONDS", "STATS_CACHE_ENABLED", "STATS_CACHE_TTL_SECONDS", "STATS_CACHE_MAX_ENTRIES"} {
		t.Setenv("ZENSTATS_"+key, "")
	}
	if err := Load("test"); err != nil {
		t.Fatal(err)
	}
	if Conf.TrustedProxies == nil || len(Conf.TrustedProxies) != 0 || Conf.RateLimit.AuthPerMinute != 15 || Conf.Clickhouse.EventsTTLDays != 730 || Conf.Clickhouse.SessionsTTLDays != 730 {
		t.Fatalf("bad defaults: %+v", Conf)
	}
	expected := StatsConfig{MaxConcurrent: 8, MaxThreads: 2, MaxMemoryUsage: 268435456, MaxExecutionTime: 20, QueryTimeoutSeconds: 30, CacheEnabled: true, CacheTTLSeconds: 15, CacheMaxEntries: 1000}
	if Conf.Stats != expected {
		t.Fatalf("got %+v want %+v", Conf.Stats, expected)
	}
	for key, value := range map[string]string{
		"TRUSTED_PROXIES":            "127.0.0.1, 10.0.0.0/8\t::1\nfc00::/7",
		"RATE_LIMIT_AUTH_PER_MINUTE": "7", "CLICKHOUSE_EVENTS_TTL_DAYS": "0", "CLICKHOUSE_SESSIONS_TTL_DAYS": "90",
		"STATS_MAX_CONCURRENT": "3", "STATS_MAX_THREADS": "4", "STATS_MAX_MEMORY_USAGE": "536870912",
		"STATS_MAX_EXECUTION_TIME": "11", "STATS_QUERY_TIMEOUT_SECONDS": "12", "STATS_CACHE_ENABLED": "false",
		"STATS_CACHE_TTL_SECONDS": "0", "STATS_CACHE_MAX_ENTRIES": "25",
	} {
		t.Setenv("ZENSTATS_"+key, value)
	}
	if err := Load("test"); err != nil {
		t.Fatal(err)
	}
	if len(Conf.TrustedProxies) != 4 || Conf.TrustedProxies[2] != "::1" || Conf.RateLimit.AuthPerMinute != 7 || Conf.Clickhouse.EventsTTLDays != 0 || Conf.Clickhouse.SessionsTTLDays != 90 {
		t.Fatalf("bad overrides: %+v", Conf)
	}
	expected = StatsConfig{MaxConcurrent: 3, MaxThreads: 4, MaxMemoryUsage: 536870912, MaxExecutionTime: 11, QueryTimeoutSeconds: 12, CacheEnabled: false, CacheTTLSeconds: 0, CacheMaxEntries: 25}
	if got := StatsSnapshot(); got != expected {
		t.Fatalf("got %+v want %+v", got, expected)
	}
	if files, err := filepath.Glob("config/config_*.yaml"); err != nil || len(files) != 0 {
		t.Fatalf("generated config: %v %v", files, err)
	}
}
