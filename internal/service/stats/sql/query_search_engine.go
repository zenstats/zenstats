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
func (qs *QueryBuilder) getSourceClause(userID int64) (string, error) {
	searchEngines := getSearchEngines()

	if userID > 0 {
		userEngines := getUserSearchEngines(userID)
		if len(userEngines) > 0 {
			merged := mergeSearchEngines(searchEngines, userEngines)
			clause := fmt.Sprintf(`
				CASE
					WHEN referrer_source = '' THEN 'Direct / None'
					%s
					ELSE referrer_source
				END
			`, qs.buildMergedSearchEngineCase(merged))
			return clause, nil
		}
	}

	clause := fmt.Sprintf(`
		CASE
			WHEN referrer_source = '' THEN 'Direct / None'
			%s
			ELSE referrer_source
		END
	`, qs.buildSearchEngineCase(searchEngines))
	return clause, nil
}

func (qs *QueryBuilder) buildSearchEngineCase(searchEngines []*ent.SearchEngines) string {
	var conditions []string
	for _, searchEngine := range searchEngines {
		conditions = append(conditions, fmt.Sprintf("WHEN positionCaseInsensitive(referrer_source, '%s') > 0 THEN '%s'", searchEngine.Domain, searchEngine.Name))
	}
	return strings.Join(conditions, "\n")
}

func (qs *QueryBuilder) buildMergedSearchEngineCase(engines []mergedSearchEngine) string {
	var conditions []string
	for _, e := range engines {
		conditions = append(conditions, fmt.Sprintf("WHEN positionCaseInsensitive(referrer_source, '%s') > 0 THEN '%s'", e.Domain, e.Name))
	}
	return strings.Join(conditions, "\n")
}

func (qs *QueryBuilder) buildUserSearchEngineCase(searchEngines []*ent.CustomSearchEngine) string {
	var conditions []string
	for _, searchEngine := range searchEngines {
		conditions = append(conditions, fmt.Sprintf("WHEN positionCaseInsensitive(referrer_source, '%s') > 0 THEN '%s'", searchEngine.Domain, searchEngine.Name))
	}
	return strings.Join(conditions, "\n")
}
