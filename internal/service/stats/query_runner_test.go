package stats

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/zenstats/zenstats/config"
	"github.com/zenstats/zenstats/internal/service/stats/types"
)

type runnerConn struct {
	driver.Conn
	calls atomic.Int32
	query func(context.Context, string, []any) (driver.Rows, error)
}

func (c *runnerConn) Query(ctx context.Context, sql string, args ...any) (driver.Rows, error) {
	c.calls.Add(1)
	if c.query != nil {
		return c.query(ctx, sql, args)
	}
	return &runnerRows{}, nil
}

type runnerColumn struct{ driver.ColumnType }

func (runnerColumn) DatabaseTypeName() string { return "UInt64" }

type runnerRows struct {
	driver.Rows
	seen bool
}

func (r *runnerRows) Next() bool {
	if r.seen {
		return false
	}
	r.seen = true
	return true
}
func (*runnerRows) Scan(dest ...any) error           { *dest[0].(*uint64) = 9007199254740993; return nil }
func (*runnerRows) Columns() []string                { return []string{"visitors"} }
func (*runnerRows) ColumnTypes() []driver.ColumnType { return []driver.ColumnType{runnerColumn{}} }
func (*runnerRows) Close() error                     { return nil }
func (*runnerRows) Err() error                       { return nil }

func runnerConfig(t *testing.T) {
	t.Helper()
	old := config.Conf
	copy := *old
	config.Conf = &copy
	config.Conf.Stats = config.StatsConfig{MaxConcurrent: 2, MaxThreads: 3, MaxMemoryUsage: 134217728, MaxExecutionTime: 9, QueryTimeoutSeconds: 2, CacheEnabled: true, CacheTTLSeconds: 1, CacheMaxEntries: 10}
	ResetQueryCache()
	t.Cleanup(func() { ResetQueryCache(); config.Conf = old })
}
func runnerQuery(siteID string) (*types.Query, *types.Site) {
	now := time.Unix(1730000000, 0).UTC()
	return &types.Query{SiteID: siteID, Timezone: "UTC", Metrics: []string{"visitors"}, Now: now, UTCTimeRange: types.TimeRange{Start: now.Add(-time.Hour), End: now}}, &types.Site{ID: siteID, Timezone: "UTC"}
}
func runForSite(t *testing.T, r *QueryRunner, siteID string) *types.QueryResult {
	t.Helper()
	q, s := runnerQuery(siteID)
	result, err := r.RunQuery(context.Background(), q, s)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func assertBusy(t *testing.T, err error) {
	t.Helper()
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.StatusCode != 503 {
		t.Fatalf("expected 503, got %v", err)
	}
}

func TestRunQueryCacheTTLAndIsolation(t *testing.T) {
	runnerConfig(t)
	conn := &runnerConn{}
	r := &QueryRunner{conn: conn}
	first := runForSite(t, r, "1")
	if first.Data[0]["visitors"] != uint64(9007199254740993) {
		t.Fatal(first)
	}
	first.Columns[0] = "changed"
	first.Data[0]["visitors"] = uint64(0)
	second := runForSite(t, r, "1")
	if conn.calls.Load() != 1 || second.Columns[0] != "visitors" || second.Data[0]["visitors"] != uint64(9007199254740993) {
		t.Fatalf("cache alias/precision error: %+v calls=%d", second, conn.calls.Load())
	}
	second.Data[0]["visitors"] = uint64(1)
	if runForSite(t, r, "1").Data[0]["visitors"] != uint64(9007199254740993) {
		t.Fatal("cache-hit return aliases cache")
	}
	runForSite(t, r, "2")
	if conn.calls.Load() != 2 {
		t.Fatal("site IDs not isolated")
	}
	time.Sleep(1050 * time.Millisecond)
	runForSite(t, r, "1")
	if conn.calls.Load() != 3 {
		t.Fatal("expired cache reused")
	}
	ResetQueryCache()
	runForSite(t, r, "1")
	if conn.calls.Load() != 4 {
		t.Fatal("reset ignored")
	}
}

func TestRunQuerySingleflight(t *testing.T) {
	runnerConfig(t)
	entered := make(chan struct{}, 1)
	unblock := make(chan struct{})
	conn := &runnerConn{query: func(ctx context.Context, _ string, _ []any) (driver.Rows, error) {
		select {
		case entered <- struct{}{}:
		default:
		}
		select {
		case <-unblock:
			return &runnerRows{}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}}
	r := &QueryRunner{conn: conn}
	const count = 40
	results := make(chan *types.QueryResult, count)
	errs := make(chan error, count)
	var started sync.WaitGroup
	started.Add(count)
	for i := 0; i < count; i++ {
		go func() {
			q, s := runnerQuery("1")
			started.Done()
			result, err := r.RunQuery(context.Background(), q, s)
			results <- result
			errs <- err
		}()
	}
	<-entered
	started.Wait()
	close(unblock)
	for i := 0; i < count; i++ {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	if got := conn.calls.Load(); got != 1 {
		t.Fatalf("underlying queries=%d want=1", got)
	}
	first := <-results
	first.Data[0]["visitors"] = uint64(0)
	for i := 1; i < count; i++ {
		if (<-results).Data[0]["visitors"] != uint64(9007199254740993) {
			t.Fatal("followers share mutable results")
		}
	}
}

func TestRunQuerySettingsAndTimeout(t *testing.T) {
	runnerConfig(t)
	config.Conf.Stats.CacheEnabled = false
	config.Conf.Stats.QueryTimeoutSeconds = 1
	conn := &runnerConn{query: func(ctx context.Context, sql string, _ []any) (driver.Rows, error) {
		for _, setting := range []string{"max_threads=3", "max_memory_usage=134217728", "max_bytes_before_external_group_by=33554432", "max_execution_time=9"} {
			if !strings.Contains(sql, setting) {
				t.Errorf("missing %s in %s", setting, sql)
			}
		}
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > time.Second {
			t.Error("query deadline not configured")
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	q, s := runnerQuery("1")
	_, err := (&QueryRunner{conn: conn}).RunQuery(context.Background(), q, s)
	assertBusy(t, err)
}

func TestRunQueryBypassLRUAndErrors(t *testing.T) {
	runnerConfig(t)
	conn := &runnerConn{}
	r := &QueryRunner{conn: conn}
	for _, mode := range []string{"disabled", "zero_ttl"} {
		config.Conf.Stats.CacheEnabled = mode != "disabled"
		config.Conf.Stats.CacheTTLSeconds = 1
		if mode == "zero_ttl" {
			config.Conf.Stats.CacheTTLSeconds = 0
		}
		before := conn.calls.Load()
		runForSite(t, r, "1")
		runForSite(t, r, "1")
		if conn.calls.Load()-before != 2 {
			t.Fatalf("%s should bypass cache", mode)
		}
	}
	config.Conf.Stats.CacheEnabled = true
	config.Conf.Stats.CacheTTLSeconds = 10
	config.Conf.Stats.CacheMaxEntries = 1
	before := conn.calls.Load()
	runForSite(t, r, "1")
	runForSite(t, r, "2")
	runForSite(t, r, "1")
	if conn.calls.Load()-before != 3 {
		t.Fatal("LRU did not evict")
	}
	ResetQueryCache()
	conn.query = func(context.Context, string, []any) (driver.Rows, error) {
		return nil, errors.New("database unavailable")
	}
	q, s := runnerQuery("3")
	if _, err := r.RunQuery(context.Background(), q, s); err == nil {
		t.Fatal("expected error")
	}
	conn.query = nil
	runForSite(t, r, "3")
	if got := conn.calls.Load(); got != before+5 {
		t.Fatalf("error cached: %d", got)
	}
}

func TestQuerySlotsLimitAndReload(t *testing.T) {
	runnerConfig(t)
	first, err := AcquireQuerySlot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer first()
	second, err := AcquireQuerySlot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer second()
	config.Conf.Stats.MaxConcurrent = 1
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = AcquireQuerySlot(ctx)
	assertBusy(t, err)
	first()
	first() // Release is idempotent.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel2()
	_, err = AcquireQuerySlot(ctx2)
	assertBusy(t, err)
	second()
	release, err := AcquireQuerySlot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	release()
}

func TestRunQueryQueueTimeoutDoesNotQuery(t *testing.T) {
	runnerConfig(t)
	config.Conf.Stats.MaxConcurrent = 1
	release, err := AcquireQuerySlot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	conn := &runnerConn{}
	q, s := runnerQuery("1")
	_, err = (&QueryRunner{conn: conn}).RunQuery(ctx, q, s)
	assertBusy(t, err)
	if conn.calls.Load() != 0 {
		t.Fatal("query ran without slot")
	}
}

func TestQuerySlotsConcurrentBudget(t *testing.T) {
	runnerConfig(t)
	config.Conf.Stats.MaxConcurrent = 3
	var active, maxActive atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			release, err := AcquireQuerySlot(ctx)
			if err != nil {
				t.Error(err)
				return
			}
			n := active.Add(1)
			for old := maxActive.Load(); n > old; old = maxActive.Load() {
				if maxActive.CompareAndSwap(old, n) {
					break
				}
			}
			time.Sleep(time.Millisecond)
			active.Add(-1)
			release()
		}()
	}
	wg.Wait()
	if n := maxActive.Load(); n > 3 || n < 2 {
		t.Fatalf("unexpected concurrency %d", n)
	}
}

func TestQueryCacheNestedDeepCopy(t *testing.T) {
	value := uint64(9007199254740993)
	original := &types.QueryResult{Columns: []string{"x"}, TotalRows: 2, Data: []map[string]any{{"map": map[string]any{"list": []any{map[string]any{"v": &value}}}, "typed": []uint64{value}, "bytes": []byte("ok")}}}
	copy := cloneQueryResult(original)
	*copy.Data[0]["map"].(map[string]any)["list"].([]any)[0].(map[string]any)["v"].(*uint64) = 1
	copy.Data[0]["typed"].([]uint64)[0] = 2
	copy.Data[0]["bytes"].([]byte)[0] = 'x'
	if value != 9007199254740993 || original.Data[0]["typed"].([]uint64)[0] != value || string(original.Data[0]["bytes"].([]byte)) != "ok" {
		t.Fatal("nested values alias")
	}
}

func TestSingleflightFollowerCancellation(t *testing.T) {
	runnerConfig(t)
	cfg := effectiveStatsConfig()
	started := make(chan struct{})
	unblock := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		_, _ = cachedQuery(context.Background(), "same", cfg, func() (*types.QueryResult, error) { close(started); <-unblock; return &types.QueryResult{}, nil })
	}()
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := cachedQuery(ctx, "same", cfg, func() (*types.QueryResult, error) { t.Error("duplicate run"); return nil, nil })
	assertBusy(t, err)
	close(unblock)
	<-finished
	if _, err := cachedQuery(context.Background(), "same", cfg, func() (*types.QueryResult, error) { t.Error("leader result not cached"); return nil, nil }); err != nil {
		t.Fatal(err)
	}
}

func TestQueryCacheKeyAndFlightCapacity(t *testing.T) {
	runnerConfig(t)
	cfg := effectiveStatsConfig()
	cfg.CacheMaxEntries = 1
	conn := &runnerConn{}
	key := queryCacheKey("1", "SELECT ?", []any{1}, nil, conn)
	for _, other := range []string{
		queryCacheKey("2", "SELECT ?", []any{1}, nil, conn),
		queryCacheKey("1", "SELECT ? + 1", []any{1}, nil, conn),
		queryCacheKey("1", "SELECT ?", []any{2}, nil, conn),
		queryCacheKey("1", "SELECT ?", []any{1}, []string{"event:page"}, conn),
		queryCacheKey("1", "SELECT ?", []any{1}, nil, &runnerConn{}),
	} {
		if key == other {
			t.Fatal("cache key collision")
		}
	}
	started := make(chan struct{})
	unblock := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		_, _ = cachedQuery(context.Background(), key, cfg, func() (*types.QueryResult, error) { close(started); <-unblock; return &types.QueryResult{}, nil })
	}()
	<-started
	_, err := cachedQuery(context.Background(), "different", cfg, func() (*types.QueryResult, error) { t.Error("capacity bypass"); return nil, nil })
	assertBusy(t, err)
	close(unblock)
	<-finished
}

// requestQuery exercises the same Params -> CreateQuery path as HTTP statistics,
// substituting a deterministic per-request clock after CreateQuery fills Now.
func requestQuery(t *testing.T, period string, now time.Time) *types.Query {
	t.Helper()
	query, err := NewQueryService(nil).CreateQuery(&types.Params{
		SiteID: "1", Timezone: "UTC", Period: period, Date: now.Format(time.DateOnly),
		Metrics: []string{"visitors"},
	})
	if err != nil {
		t.Fatal(err)
	}
	query.Now = now
	if period == "realtime" {
		// Both realtime constructors read the clock before CreateQuery sets Now.
		query.UTCTimeRange.End = now.Add(-time.Microsecond)
		query.UTCTimeRange.Start = query.UTCTimeRange.End.Add(-30 * time.Minute)
	}
	return query
}

func assertQueryTimeArg(t *testing.T, args []any, want time.Time) {
	t.Helper()
	for _, arg := range args {
		if value, ok := arg.(time.Time); ok && value.Equal(want) {
			return
		}
	}
	t.Errorf("SQL args %v do not contain boundary %s", args, want.Format(time.RFC3339Nano))
}

func TestRunQueryRelativeTimeBucketSingleflight(t *testing.T) {
	for _, period := range []string{"p30", "realtime"} {
		t.Run(period, func(t *testing.T) {
			runnerConfig(t)
			config.Conf.Stats.CacheTTLSeconds = 15
			bucket := 15 * time.Second
			base := time.Date(2026, 11, 26, 12, 0, 1, 123456789, time.UTC)
			upper := base.Truncate(bucket).Add(bucket)
			queries := []*types.Query{
				requestQuery(t, period, base),
				requestQuery(t, period, base.Add(200*time.Millisecond)),
			}
			snapshots := []*types.Query{
				cloneQueryValue(reflect.ValueOf(queries[0])).Interface().(*types.Query),
				cloneQueryValue(reflect.ValueOf(queries[1])).Interface().(*types.Query),
			}
			entered := make(chan struct{}, 1)
			unblock := make(chan struct{})
			conn := &runnerConn{query: func(ctx context.Context, _ string, args []any) (driver.Rows, error) {
				assertQueryTimeArg(t, args, upper)
				if period == "realtime" {
					assertQueryTimeArg(t, args, upper.Add(-30*time.Minute))
				}
				select {
				case entered <- struct{}{}:
				default:
				}
				select {
				case <-unblock:
					return &runnerRows{}, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}}
			runner := &QueryRunner{conn: conn}
			site := &types.Site{ID: "1", Timezone: "UTC"}
			errors := make(chan error, 10)
			var started sync.WaitGroup
			started.Add(10)
			for i := 0; i < 10; i++ {
				go func(i int) {
					started.Done()
					_, err := runner.RunQuery(context.Background(), queries[i%2], site)
					errors <- err
				}(i)
			}
			<-entered
			started.Wait()
			close(unblock)
			for i := 0; i < 10; i++ {
				if err := <-errors; err != nil {
					t.Fatal(err)
				}
			}
			// A fresh HTTP request after the burst must reuse the completed result.
			followup := requestQuery(t, period, base.Add(400*time.Millisecond))
			if _, err := runner.RunQuery(context.Background(), followup, site); err != nil {
				t.Fatal(err)
			}
			if got := conn.calls.Load(); got != 1 {
				t.Fatalf("10 concurrent + 1 followup executed %d queries, want 1", got)
			}
			for i := range queries {
				if !reflect.DeepEqual(queries[i], snapshots[i]) {
					t.Errorf("caller query %d was mutated", i)
				}
			}
			// Crossing a time bucket must refresh the moving window even within cache TTL.
			next := requestQuery(t, period, base.Add(bucket))
			conn.query = func(_ context.Context, _ string, args []any) (driver.Rows, error) {
				assertQueryTimeArg(t, args, upper.Add(bucket))
				return &runnerRows{}, nil
			}
			if _, err := runner.RunQuery(context.Background(), next, site); err != nil {
				t.Fatal(err)
			}
			if got := conn.calls.Load(); got != 2 {
				t.Fatalf("new bucket reused old result: %d", got)
			}
		})
	}
}

func TestRunQueryTimeBucketPreservesFixedBounds(t *testing.T) {
	runnerConfig(t)
	config.Conf.Stats.CacheTTLSeconds = 15
	now := time.Date(2026, 11, 26, 12, 0, 1, 123456789, time.UTC)
	for _, end := range []time.Time{
		now.Add(-time.Hour),  // Historical interval must remain exact.
		now.Add(time.Second), // Original upper bound before the bucket ceiling still wins.
	} {
		query := requestQuery(t, "p30", now)
		query.UTCTimeRange.End = end
		query.Dimensions = []string{"time"} // The optimizer also mutates slices/order entries.
		query.OrderBy = []*types.OrderBy{{Dimension: "time", Direction: "asc"}}
		original := cloneQueryValue(reflect.ValueOf(query)).Interface().(*types.Query)
		conn := &runnerConn{query: func(_ context.Context, _ string, args []any) (driver.Rows, error) {
			assertQueryTimeArg(t, args, original.UTCTimeRange.Start)
			assertQueryTimeArg(t, args, end)
			return &runnerRows{}, nil
		}}
		if _, err := (&QueryRunner{conn: conn}).RunQuery(context.Background(), query, &types.Site{ID: "1", Timezone: "UTC"}); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(query, original) {
			t.Fatal("caller query or nested fields mutated")
		}
	}
}

func TestRunQueryWithoutCacheKeepsNanoseconds(t *testing.T) {
	for _, mode := range []string{"disabled", "zero_ttl"} {
		t.Run(mode, func(t *testing.T) {
			runnerConfig(t)
			config.Conf.Stats.CacheEnabled = mode != "disabled"
			if mode == "zero_ttl" {
				config.Conf.Stats.CacheTTLSeconds = 0
			}
			for _, period := range []string{"p30", "realtime"} {
				for _, nanos := range []int{123456789, 323456789} {
					now := time.Date(2026, 11, 26, 12, 0, 1, nanos, time.UTC)
					query := requestQuery(t, period, now)
					original := cloneQueryValue(reflect.ValueOf(query)).Interface().(*types.Query)
					wantEnd := now
					if period == "realtime" {
						wantEnd = query.UTCTimeRange.End
					}
					conn := &runnerConn{query: func(_ context.Context, _ string, args []any) (driver.Rows, error) {
						assertQueryTimeArg(t, args, wantEnd)
						assertQueryTimeArg(t, args, original.UTCTimeRange.Start)
						return &runnerRows{}, nil
					}}
					if _, err := (&QueryRunner{conn: conn}).RunQuery(context.Background(), query, &types.Site{ID: "1", Timezone: "UTC"}); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(query, original) {
						t.Fatal("uncached caller query mutated")
					}
				}
			}
		})
	}
}
