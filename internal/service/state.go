package service

import (
	"context"
	"fmt"
	"sync"

	cl "github.com/zenstats/zenstats/internal/store/clickhouse"
	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/pkg/globals"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

var (
	stateServiceInstance *StateService
	stateOnce            sync.Once
)

type TopStats struct {
	PV            uint64
	UV            uint64
	Sessions      uint64
	PrevPV        uint64
	PrevUV        uint64
	PrevSessions  uint64
	PVChange      float64
	UVChange      float64
	SessionChange float64
}

type StateService struct {
	db *postgresql.Client
	cl driver.Conn
}

func GetStateService() *StateService {
	stateOnce.Do(func() {
		db := globals.GetDB()
		if db == nil {
			panic("DB is not initialized")
		}
		stateServiceInstance = &StateService{
			db: db,
			cl: cl.GetConnection(),
		}
	})
	return stateServiceInstance
}

func (s *StateService) GetTopStats() (*TopStats, error) {
	query := `
		SELECT
			count(*) as pv,
			count(distinct user_id) as uv,
			(SELECT count(distinct session_id) FROM zenstats_events_db.sessions WHERE toDate(timestamp) BETWEEN today() - 1 AND today()) as sessions,
			(SELECT count(*) FROM zenstats_events_db.events WHERE name = 'pageview' AND toDate(timestamp) BETWEEN today() - 14 AND today() - 8) as prev_pv,
			(SELECT count(distinct user_id) FROM zenstats_events_db.events WHERE  name = 'pageview' AND toDate(timestamp) BETWEEN today() - 14 AND today() - 8) as prev_uv,
			(SELECT count(distinct session_id) FROM zenstats_events_db.sessions WHERE toDate(timestamp) BETWEEN today() - 14 AND today() - 8) as prev_sessions
		FROM
			zenstats_events_db.events
		WHERE
			toDate(timestamp) BETWEEN today() - 7 AND today() - 1
	`
	var stats TopStats
	err := s.cl.QueryRow(context.Background(), query).Scan(
		&stats.PV,
		&stats.UV,
		&stats.Sessions,
		&stats.PrevPV,
		&stats.PrevUV,
		&stats.PrevSessions,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	// 计算变化值
	if stats.PrevPV > 0 {
		stats.PVChange = float64(stats.PV-stats.PrevPV) / float64(stats.PrevPV) * 100
	}
	if stats.PrevUV > 0 {
		stats.UVChange = float64(stats.UV-stats.PrevUV) / float64(stats.PrevUV) * 100
	}
	if stats.PrevSessions > 0 {
		stats.SessionChange = float64(stats.Sessions-stats.PrevSessions) / float64(stats.PrevSessions) * 100
	}

	return &stats, nil
}
