package stats

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/zenstats/zenstats/internal/service/stats/types"
	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/pkg/globals"
)

func TestCreateQuery(t *testing.T) {
	globals.SetDB(postgresql.NewClient())

	tests := []struct {
		name    string
		params  *types.Params
		wantErr bool
	}{
		{
			name: "valid query",
			params: &types.Params{
				SiteID:   "1",
				Period:   "day",
				Date:     "2025-08-12",
				Timezone: "Asia/Shanghai",
				// UTCTimeRange: types.TimeRange{
				// 	Start: time.Now().Add(-24 * time.Hour),
				// 	End:   time.Now(),
				// },
				Metrics: []string{"visitors"},
				// Dimensions: []string{"visit:source", "visit:browser"},
				Dimensions: []string{"visit:source"},
				// Filters: []*types.Filter{{
				// 	Operator: "or",
				// 	SubFilters: []*types.Filter{
				// 		&types.Filter{
				// 			Operator:  "contains",
				// 			Dimension: "visit:country",
				// 			Values:    []any{"CN"},
				// 		},
				// 	},
				// }},
				Pagination: &types.Pagination{
					Limit:  10,
					Offset: 0,
				},
			},
			wantErr: false,
		},
	}

	// 创建测试用QueryRunner实例
	runner := NewQueryRunner()
	qs := NewQueryService(runner)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, err := qs.CreateQuery(tt.params)
			if err != nil {
				t.Errorf("CreateQuery() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && query == nil {
				t.Error("CreateQuery() returned nil query for valid input")
			}
			if !tt.wantErr && query.Now.IsZero() {
				t.Error("CreateQuery() did not set Now field")
			}
			if !tt.wantErr {
				site := &types.Site{ID: query.SiteID, UserID: query.UserID, Timezone: query.Timezone}
				result, err := qs.Execute(context.Background(), query, site)
				if err != nil {
					t.Errorf("Execute() error = %v", err)
					return
				}
				if result == nil {
					t.Error("Execute() returned nil result")
				}
				// json打印result
				jsonResult, err := json.Marshal(result)
				if err != nil {
					t.Errorf("Marshal() error = %v", err)
					return
				}
				fmt.Println(string(jsonResult))
			}
		})
	}
}
