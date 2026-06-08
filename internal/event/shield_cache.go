package event

import (
	"sync"
	"time"

	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
)

// ShieldRulesCache provides in-memory caching for shield rules to reduce DB queries.
type ShieldRulesCache struct {
	hostnameCache sync.Map // key: domain -> *shieldHostnameCacheEntry
	ipCache       sync.Map // key: domain -> *shieldIPCacheEntry
	countryCache  sync.Map // key: domain -> *shieldCountryCacheEntry
}

type shieldHostnameCacheEntry struct {
	rules     []*ent.ShieldRulesHostname
	expiresAt time.Time
}

type shieldIPCacheEntry struct {
	rules     []*ent.ShieldRulesIp
	expiresAt time.Time
}

type shieldCountryCacheEntry struct {
	rules     []*ent.ShieldRulesCountry
	expiresAt time.Time
}

var (
	shieldCacheInstance *ShieldRulesCache
	shieldCacheOnce     sync.Once
)

const shieldCacheTTL = 5 * time.Minute

// GetShieldRulesCache returns the singleton shield rules cache instance.
func GetShieldRulesCache() *ShieldRulesCache {
	shieldCacheOnce.Do(func() {
		shieldCacheInstance = &ShieldRulesCache{}
	})
	return shieldCacheInstance
}

// InvalidateDomain clears all cached shield rules for a specific domain.
func (c *ShieldRulesCache) InvalidateDomain(domain string) {
	c.hostnameCache.Delete(domain)
	c.ipCache.Delete(domain)
	c.countryCache.Delete(domain)
}

// GetHostnameRules returns cached hostname rules or fetches from the loader.
func (c *ShieldRulesCache) GetHostnameRules(domain string, loader func() ([]*ent.ShieldRulesHostname, error)) ([]*ent.ShieldRulesHostname, error) {
	if entry, ok := c.hostnameCache.Load(domain); ok {
		cached := entry.(*shieldHostnameCacheEntry)
		if time.Now().Before(cached.expiresAt) {
			return cached.rules, nil
		}
	}

	rules, err := loader()
	if err != nil {
		return nil, err
	}

	c.hostnameCache.Store(domain, &shieldHostnameCacheEntry{
		rules:     rules,
		expiresAt: time.Now().Add(shieldCacheTTL),
	})

	return rules, nil
}

// GetIPRules returns cached IP rules or fetches from the loader.
func (c *ShieldRulesCache) GetIPRules(domain string, loader func() ([]*ent.ShieldRulesIp, error)) ([]*ent.ShieldRulesIp, error) {
	if entry, ok := c.ipCache.Load(domain); ok {
		cached := entry.(*shieldIPCacheEntry)
		if time.Now().Before(cached.expiresAt) {
			return cached.rules, nil
		}
	}

	rules, err := loader()
	if err != nil {
		return nil, err
	}

	c.ipCache.Store(domain, &shieldIPCacheEntry{
		rules:     rules,
		expiresAt: time.Now().Add(shieldCacheTTL),
	})

	return rules, nil
}

// GetCountryRules returns cached country rules or fetches from the loader.
func (c *ShieldRulesCache) GetCountryRules(domain string, loader func() ([]*ent.ShieldRulesCountry, error)) ([]*ent.ShieldRulesCountry, error) {
	if entry, ok := c.countryCache.Load(domain); ok {
		cached := entry.(*shieldCountryCacheEntry)
		if time.Now().Before(cached.expiresAt) {
			return cached.rules, nil
		}
	}

	rules, err := loader()
	if err != nil {
		return nil, err
	}

	c.countryCache.Store(domain, &shieldCountryCacheEntry{
		rules:     rules,
		expiresAt: time.Now().Add(shieldCacheTTL),
	})

	return rules, nil
}
