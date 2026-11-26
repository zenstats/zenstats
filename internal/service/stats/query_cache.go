package stats

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/zenstats/zenstats/config"
	"github.com/zenstats/zenstats/internal/service/stats/types"
)

type queryCacheEntry struct {
	result  *types.QueryResult
	expires time.Time
}
type queryFlight struct {
	done   chan struct{}
	result *types.QueryResult
	err    error
}
type queryCacheState struct {
	cfg     config.StatsConfig
	entries *lru.Cache[string, queryCacheEntry]
	flights map[string]*queryFlight
}

var queryCache struct {
	sync.Mutex
	state *queryCacheState
}

// ResetQueryCache invalidates stored results without disrupting active callers.
func ResetQueryCache() {
	queryCache.Lock()
	queryCache.state = nil
	queryCache.Unlock()
}

func queryCacheKey(siteID, sql string, args []any, dimensions []string, conn any) string {
	// Include result shape and connection identity to isolate runners using different databases.
	payload, err := json.Marshal([]any{siteID, sql, args, dimensions, fmt.Sprintf("%p", conn)})
	if err != nil {
		return ""
	} // Unsupported arguments must bypass caching.
	return fmt.Sprintf("%x", sha256.Sum256(payload))
}

func cachedQuery(ctx context.Context, key string, cfg config.StatsConfig, run func() (*types.QueryResult, error)) (*types.QueryResult, error) {
	if ctx.Err() != nil {
		return nil, translateQueryError(ctx.Err())
	}
	if !cfg.CacheEnabled || cfg.CacheTTLSeconds <= 0 || key == "" {
		return run()
	}
	queryCache.Lock()
	state := queryCache.state
	if state == nil || state.cfg != cfg {
		entries, _ := lru.New[string, queryCacheEntry](cfg.CacheMaxEntries)
		state = &queryCacheState{cfg: cfg, entries: entries, flights: make(map[string]*queryFlight)}
		queryCache.state = state
	}
	if entry, ok := state.entries.Get(key); ok {
		if time.Now().Before(entry.expires) {
			queryCache.Unlock()
			return cloneQueryResult(entry.result), nil
		}
		state.entries.Remove(key)
	}
	if flight, ok := state.flights[key]; ok {
		queryCache.Unlock()
		select {
		case <-ctx.Done():
			return nil, translateQueryError(ctx.Err())
		case <-flight.done:
			return cloneQueryResult(flight.result), flight.err
		}
	}
	// Bound in-flight bookkeeping too; reject new work instead of bypassing deduplication.
	if len(state.flights) >= cfg.CacheMaxEntries {
		queryCache.Unlock()
		return nil, queryQueueBusy()
	}
	flight := &queryFlight{done: make(chan struct{})}
	state.flights[key] = flight
	queryCache.Unlock()

	// Always release followers, even if a driver panics and HTTP recovery handles it.
	defer func() {
		queryCache.Lock()
		delete(state.flights, key)
		if flight.result == nil && flight.err == nil {
			flight.err = fmt.Errorf("statistics query interrupted")
		}
		close(flight.done)
		queryCache.Unlock()
	}()
	result, err := run()
	flight.err = err
	if err == nil {
		flight.result = cloneQueryResult(result)
		queryCache.Lock()
		state.entries.Add(key, queryCacheEntry{result: flight.result, expires: time.Now().Add(time.Duration(cfg.CacheTTLSeconds) * time.Second)})
		queryCache.Unlock()
	}
	return result, err
}

func cloneQueryResult(result *types.QueryResult) *types.QueryResult {
	if result == nil {
		return nil
	}
	return cloneQueryValue(reflect.ValueOf(result)).Interface().(*types.QueryResult)
}

// Preserve numeric types and nested ClickHouse arrays/maps; JSON round-tripping
// would turn UInt64 values into imprecise float64 values.
func cloneQueryValue(value reflect.Value) reflect.Value {
	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		copy := reflect.New(value.Type()).Elem()
		if value.Kind() == reflect.Pointer {
			copy.Set(reflect.New(value.Type().Elem()))
			copy.Elem().Set(cloneQueryValue(value.Elem()))
		} else {
			copy.Set(cloneQueryValue(value.Elem()))
		}
		return copy
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		copy := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := 0; i < value.Len(); i++ {
			copy.Index(i).Set(cloneQueryValue(value.Index(i)))
		}
		return copy
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		copy := reflect.MakeMapWithSize(value.Type(), value.Len())
		iter := value.MapRange()
		for iter.Next() {
			copy.SetMapIndex(iter.Key(), cloneQueryValue(iter.Value()))
		}
		return copy
	case reflect.Array, reflect.Struct:
		copy := reflect.New(value.Type()).Elem()
		copy.Set(value)
		if value.Kind() == reflect.Array {
			for i := 0; i < value.Len(); i++ {
				copy.Index(i).Set(cloneQueryValue(value.Index(i)))
			}
		} else {
			for i := 0; i < value.NumField(); i++ {
				if copy.Field(i).CanSet() && value.Field(i).CanInterface() {
					copy.Field(i).Set(cloneQueryValue(value.Field(i)))
				}
			}
		}
		return copy
	default:
		return value
	}
}
