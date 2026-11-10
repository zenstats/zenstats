package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/gin-gonic/gin"
	atypes "github.com/zenstats/zenstats/internal/api/types"
	querystats "github.com/zenstats/zenstats/internal/service/stats"
	statssql "github.com/zenstats/zenstats/internal/service/stats/sql"
	"github.com/zenstats/zenstats/internal/service/stats/types"
)

// UserFlowsParams limits the first observed page in the selected range, not an
// arbitrary page visited later. Adjacent repeated pageviews count as one step.
type UserFlowsParams struct {
	EntryPath string
	Steps     int
	Limit     int
}

type SankeyNode struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Depth     int    `json:"depth"`
	IsDropOff bool   `json:"is_dropoff,omitempty"`
	IsOther   bool   `json:"is_other,omitempty"`
}

type SankeyLink struct {
	Source     int    `json:"source"`
	Target     int    `json:"target"`
	Value      uint64 `json:"value"`
	SourceName string `json:"source_name,omitempty"`
	TargetName string `json:"target_name,omitempty"`
}

// DropOff means no next distinct page was observed BEFORE the requested final
// step. It does not assert a browser close, a completed session or user churn.
type UserFlowsResponse struct {
	Nodes           []SankeyNode `json:"nodes"`
	Links           []SankeyLink `json:"links"`
	TotalSessions   uint64       `json:"total_sessions"`
	DropOffSessions uint64       `json:"drop_off_sessions"`
	DropOffRate     float64      `json:"drop_off_rate"`
}

type rawFlowEdge struct {
	Depth       uint8  `ch:"depth"`
	Source      string `ch:"source"`
	SourceOther uint8  `ch:"source_other"`
	Target      string `ch:"target"`
	TargetOther uint8  `ch:"target_other"`
	IsDropOff   uint8  `ch:"is_dropoff"`
	Count       uint64 `ch:"session_count"`
}

func (s *StatsService) GetUserFlows(c *gin.Context, siteID int64, req *atypes.StatsRequest, flow UserFlowsParams) (*UserFlowsResponse, error) {
	ctx := c.Request.Context()
	site, err := GetSiteService().GetSiteByID(c, int(siteID))
	if err != nil {
		return nil, fmt.Errorf("site not found")
	}
	filters, err := parseRequestFilters(req.Filters)
	if err != nil {
		return nil, &querystats.ValidationError{StatusCode: 400, Message: "invalid flow filters: " + err.Error()}
	}
	// Reuse statistics validation and date parsing: unknown periods must never
	// fall through to an unbounded zero-date scan.
	query, err := s.getQueryService().CreateQuery(&types.Params{
		SiteID: strconv.FormatInt(siteID, 10), UserID: c.GetInt64("user_id"),
		Period: req.Period, Date: req.Date, From: req.From, To: req.To,
		Timezone: site.Timezone, Metrics: []string{"visitors"}, Filters: filters,
	})
	if err != nil {
		return nil, &querystats.ValidationError{StatusCode: 400, Message: err.Error()}
	}
	if query.UTCTimeRange.Start.After(query.UTCTimeRange.End) {
		return nil, &querystats.ValidationError{StatusCode: 400, Message: "start time must be before end time"}
	}
	if flow.Steps == 0 {
		flow.Steps = 3
	}
	if flow.Limit == 0 {
		flow.Limit = 6
	}
	if flow.Steps < 2 || flow.Steps > 4 || flow.Limit < 3 || flow.Limit > 12 {
		return nil, &querystats.ValidationError{StatusCode: 400, Message: "steps must be 2–4 and limit must be 3–12"}
	}
	edges, err := s.fetchFlowEdges(ctx, query, flow)
	if err != nil {
		return nil, err
	}
	return buildSankeyGraph(edges), nil
}

func (s *StatsService) fetchFlowEdges(ctx context.Context, query *types.Query, flow UserFlowsParams) ([]rawFlowEdge, error) {
	statement, args, err := buildUserFlowsQuery(query, flow)
	if err != nil {
		return nil, &querystats.ValidationError{StatusCode: 400, Message: err.Error()}
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	release, err := querystats.AcquireQuerySlot(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	ctx = clickhouse.Context(ctx, clickhouse.WithSettings(clickhouse.Settings{
		"max_threads": 1, "max_execution_time": 20, "max_memory_usage": 268435456,
		"max_bytes_before_external_group_by": 67108864, "max_bytes_before_external_sort": 67108864,
	}))
	var edges []rawFlowEdge
	if err := s.cl.Select(ctx, &edges, statement, args...); err != nil {
		return nil, querystats.TranslateQueryError(err)
	}
	return edges, nil
}

// The event column has second precision. Lexical pathname ordering breaks ties
// deterministically; events within the same second cannot be truly sequenced.
// Each session retains at most N page transitions. Top N is selected per depth
// using ALL sessions, before merging the remaining pages into an Other node.
// The final grouping has at most (steps-1)*(limit+1)*(limit+2) edges.
func buildUserFlowsQuery(query *types.Query, flow UserFlowsParams) (string, []any, error) {
	base := statssql.NewWhereBuilder(query.SiteID)
	base.FilterSiteTimeRange("events", query.UTCTimeRange.Start, query.UTCTimeRange.End)
	base.AddCondition("name = 'pageview' AND session_id != 0 AND pathname != ''")
	if len(query.Filters) > 0 {
		cohort := statssql.NewWhereBuilder(query.SiteID)
		cohort.SetUserID(query.UserID)
		cohort.FilterSiteTimeRange("events", query.UTCTimeRange.Start, query.UTCTimeRange.End)
		// Select matching sessions, then read their complete in-range navigation.
		// Applying a page/event filter directly would erase intermediate pages.
		if err := cohort.AddFilter("events", &types.Filter{Operator: "and", SubFilters: query.Filters}); err != nil {
			return "", nil, err
		}
		where, args := cohort.Build()
		base.AddCondition("(user_id, session_id) IN (SELECT user_id, session_id FROM events WHERE "+where+")", args...)
	}
	where, args := base.Build()
	entry := ""
	if flow.EntryPath != "" {
		entry = " HAVING steps[1] = ?"
		args = append(args, flow.EntryPath)
	}
	// Only bounded, validated integers are interpolated. Paths and filter values
	// remain parameters. Every CTE is referenced once: one navigation scan.
	statement := fmt.Sprintf(`WITH ordered_pageviews AS (
  SELECT user_id, session_id, timestamp, pathname,
   lagInFrame(pathname, 1, '') OVER (
    PARTITION BY user_id, session_id ORDER BY timestamp, pathname
    ROWS BETWEEN 1 PRECEDING AND CURRENT ROW) AS previous_path
  FROM events WHERE %s
 ), session_steps AS (
  SELECT user_id, session_id,
   arrayMap(x -> x.2, groupArraySorted(%d)(tuple(timestamp, pathname))) AS steps
  FROM ordered_pageviews WHERE pathname != previous_path
  GROUP BY user_id, session_id%s
 ), step_visits AS (
  SELECT user_id, session_id, visit.1 AS depth, visit.2 AS page
  FROM session_steps ARRAY JOIN arrayZip(arrayEnumerate(steps), steps) AS visit
 ), page_volumes AS (
  SELECT *, count() OVER (PARTITION BY depth, page) AS volume FROM step_visits
 ), ranked_visits AS (
  SELECT *, dense_rank() OVER (PARTITION BY depth ORDER BY volume DESC, page) AS page_rank
  FROM page_volumes
 ), grouped_steps AS (
  SELECT user_id, session_id,
   arrayMap(x -> tuple(x.2, x.3), groupArraySorted(%d)(tuple(depth,
    if(page_rank <= %d, page, ''), toUInt8(page_rank > %d)))) AS steps
  FROM ranked_visits GROUP BY user_id, session_id
 )
 SELECT toUInt8(position) AS depth,
  steps[position+1].1 AS source, steps[position+1].2 AS source_other,
  steps[position+2].1 AS target, steps[position+2].2 AS target_other,
  toUInt8(position+1 >= length(steps)) AS is_dropoff, count() AS session_count
 FROM grouped_steps
 ARRAY JOIN range(toUInt64(least(length(steps), %d))) AS position
 GROUP BY depth, source, source_other, target, target_other, is_dropoff`,
		where, flow.Steps, entry, flow.Steps, flow.Limit, flow.Limit, flow.Steps-1)
	return statement, args, nil
}

func buildSankeyGraph(rows []rawFlowEdge) *UserFlowsResponse {
	result := &UserFlowsResponse{Nodes: []SankeyNode{}, Links: []SankeyLink{}}
	type nodeKey struct {
		depth       int
		page        string
		other, exit bool
	}
	type edgeKey struct{ source, target nodeKey }
	nodes := map[nodeKey]bool{}
	edges := map[edgeKey]uint64{}
	for _, row := range rows {
		if row.Count == 0 {
			continue
		}
		source := nodeKey{depth: int(row.Depth), page: row.Source, other: row.SourceOther != 0}
		target := nodeKey{depth: int(row.Depth) + 1, page: row.Target, other: row.TargetOther != 0, exit: row.IsDropOff != 0}
		nodes[source], nodes[target] = true, true
		edges[edgeKey{source, target}] += row.Count
		if row.Depth == 0 {
			result.TotalSessions += row.Count
		}
		if target.exit {
			result.DropOffSessions += row.Count
		}
	}
	keys := make([]nodeKey, 0, len(nodes))
	for key := range nodes {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.depth != b.depth {
			return a.depth < b.depth
		}
		if a.exit != b.exit {
			return !a.exit
		}
		if a.other != b.other {
			return !a.other
		}
		return a.page < b.page
	})
	indices := make(map[nodeKey]int, len(keys))
	for _, key := range keys {
		name, kind := key.page, "page"
		if key.other {
			name, kind = "(other)", "other"
		}
		if key.exit {
			name, kind = "(exit)", "exit"
		}
		indices[key] = len(result.Nodes)
		result.Nodes = append(result.Nodes, SankeyNode{ID: fmt.Sprintf("%d:%s:%s", key.depth, kind, key.page), Name: name, Depth: key.depth, IsOther: key.other, IsDropOff: key.exit})
	}
	for key, count := range edges {
		source, target := indices[key.source], indices[key.target]
		result.Links = append(result.Links, SankeyLink{Source: source, Target: target, Value: count, SourceName: result.Nodes[source].Name, TargetName: result.Nodes[target].Name})
	}
	sort.Slice(result.Links, func(i, j int) bool {
		a, b := result.Links[i], result.Links[j]
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		return a.Target < b.Target
	})
	if result.TotalSessions > 0 {
		result.DropOffRate = float64(result.DropOffSessions) / float64(result.TotalSessions) * 100
	}
	return result
}
