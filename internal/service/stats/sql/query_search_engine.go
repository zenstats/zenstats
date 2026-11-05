package sql

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/customsearchengine"
	"github.com/zenstats/zenstats/pkg/globals"
)

// ---------------------------------------------------------------------------
// Search engine cache
// ---------------------------------------------------------------------------

var (
	searchEngineCache     []*ent.SearchEngines
	searchEngineCacheTime time.Time
	searchEngineCacheMu   sync.RWMutex
	searchEngineCacheTTL  = 5 * time.Minute
)

var (
	userSearchEngineCache     = make(map[int64][]*ent.CustomSearchEngine)
	userSearchEngineCacheTime = make(map[int64]time.Time)
	userSearchEngineCacheMu   sync.RWMutex
	userSearchEngineCacheTTL  = 5 * time.Minute
)

// InvalidateUserSearchEngineCache clears the cache for a user so changes take effect immediately.
func InvalidateUserSearchEngineCache(userID int64) {
	userSearchEngineCacheMu.Lock()
	delete(userSearchEngineCache, userID)
	delete(userSearchEngineCacheTime, userID)
	userSearchEngineCacheMu.Unlock()
}

// InvalidateSearchEngineCache clears the global search engine cache so admin changes take effect immediately.
func InvalidateSearchEngineCache() {
	searchEngineCacheMu.Lock()
	searchEngineCache = nil
	searchEngineCacheTime = time.Time{}
	searchEngineCacheMu.Unlock()
}

// ---------------------------------------------------------------------------
// Search engine data loading & CASE building
// ---------------------------------------------------------------------------

type mergedSearchEngine struct {
	Domain string
	Name   string
}

func getSearchEngines() []*ent.SearchEngines {
	searchEngineCacheMu.RLock()
	if searchEngineCache != nil && time.Since(searchEngineCacheTime) < searchEngineCacheTTL {
		defer searchEngineCacheMu.RUnlock()
		return searchEngineCache
	}
	searchEngineCacheMu.RUnlock()

	searchEngineCacheMu.Lock()
	defer searchEngineCacheMu.Unlock()

	if searchEngineCache != nil && time.Since(searchEngineCacheTime) < searchEngineCacheTTL {
		return searchEngineCache
	}

	db := globals.GetDB()
	if db == nil {
		return searchEngineCache
	}
	searchEngineCache = db.Client.SearchEngines.Query().AllX(context.Background())
	searchEngineCacheTime = time.Now()
	return searchEngineCache
}

func getUserSearchEngines(userID int64) []*ent.CustomSearchEngine {
	userSearchEngineCacheMu.RLock()
	if cache, ok := userSearchEngineCache[userID]; ok {
		if time.Since(userSearchEngineCacheTime[userID]) < userSearchEngineCacheTTL {
			defer userSearchEngineCacheMu.RUnlock()
			return cache
		}
	}
	userSearchEngineCacheMu.RUnlock()

	userSearchEngineCacheMu.Lock()
	defer userSearchEngineCacheMu.Unlock()

	if cache, ok := userSearchEngineCache[userID]; ok {
		if time.Since(userSearchEngineCacheTime[userID]) < userSearchEngineCacheTTL {
			return cache
		}
	}

	db := globals.GetDB()
	if db == nil {
		return userSearchEngineCache[userID]
	}
	engines := db.Client.CustomSearchEngine.Query().
		Where(customsearchengine.UserID(userID)).
		AllX(context.Background())
	userSearchEngineCache[userID] = engines
	userSearchEngineCacheTime[userID] = time.Now()
	return engines
}

func mergeSearchEngines(globalEngines []*ent.SearchEngines, userEngines []*ent.CustomSearchEngine) []mergedSearchEngine {
	merged := make([]mergedSearchEngine, 0, len(globalEngines)+len(userEngines))
	domainMap := make(map[string]bool)

	for _, e := range globalEngines {
		merged = append(merged, mergedSearchEngine{Domain: e.Domain, Name: e.Name})
		domainMap[e.Domain] = true
	}

	for _, e := range userEngines {
		if domainMap[e.Domain] {
			for i, m := range merged {
				if m.Domain == e.Domain {
					merged[i].Name = e.Name
					break
				}
			}
		} else {
			merged = append(merged, mergedSearchEngine{Domain: e.Domain, Name: e.Name})
			domainMap[e.Domain] = true
		}
	}

	return merged
}

// getSourceClause generates the CASE statement for source classification.
func (qs *QueryBuilder) getSourceClause(userID int64, tableAlias string) (string, error) {
	globalEngines := getSearchEngines()
	var userEngines []*ent.CustomSearchEngine
	if userID > 0 {
		userEngines = getUserSearchEngines(userID)
	}
	engines := mergeSearchEngines(globalEngines, userEngines)
	referrer := tableAlias + ".referrer_source"
	utm := tableAlias + ".utm_source"
	var conditions []string
	for _, engine := range engines {
		conditions = append(conditions, sourceDomainCase(engine.Domain, engine.Name, referrer))
	}
	return fmt.Sprintf(`CASE
		WHEN %s != '' THEN %s
		WHEN %s = '' THEN 'Direct / None'
		%s
		ELSE %s
	END`, utm, utm, referrer, strings.Join(conditions, "\n"), referrer), nil
}

func (qs *QueryBuilder) buildSearchEngineCase(searchEngines []*ent.SearchEngines) string {
	var conditions []string
	for _, searchEngine := range searchEngines {
		conditions = append(conditions, sourceDomainCase(searchEngine.Domain, searchEngine.Name, "referrer_source"))
	}
	return strings.Join(conditions, "\n")
}

func (qs *QueryBuilder) buildMergedSearchEngineCase(engines []mergedSearchEngine) string {
	var conditions []string
	for _, e := range engines {
		conditions = append(conditions, sourceDomainCase(e.Domain, e.Name, "referrer_source"))
	}
	return strings.Join(conditions, "\n")
}

func (qs *QueryBuilder) buildUserSearchEngineCase(searchEngines []*ent.CustomSearchEngine) string {
	var conditions []string
	for _, searchEngine := range searchEngines {
		conditions = append(conditions, sourceDomainCase(searchEngine.Domain, searchEngine.Name, "referrer_source"))
	}
	return strings.Join(conditions, "\n")
}

func sourceDomainCase(domain, name, column string) string {
	domain = strings.ToLower(strings.Trim(strings.TrimSpace(domain), "."))
	if domain == "" {
		return ""
	}
	return fmt.Sprintf("WHEN (lower(%s) = '%s' OR endsWith(lower(%s), '.%s')) THEN '%s'", column, escapeCHString(domain), column, escapeCHString(domain), escapeCHString(name))
}
