package service

import (
	"context"
	"sync"

	"github.com/zenstats/zenstats/internal/service/stats/sql"
	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	"github.com/zenstats/zenstats/pkg/globals"
)

// SearchEngineService manages the global built-in search engines table.
type SearchEngineService struct {
	db *postgresql.Client
}

// GetSearchEngineService returns the singleton SearchEngineService.
var GetSearchEngineService = sync.OnceValue(func() *SearchEngineService {
	db := globals.GetDB()
	if db == nil {
		panic("DB is not initialized")
	}
	return &SearchEngineService{db: db}
})

// ListAll returns all built-in search engines.
func (s *SearchEngineService) ListAll(ctx context.Context) ([]*ent.SearchEngines, error) {
	return s.db.Client.SearchEngines.Query().All(ctx)
}

// Create adds a new built-in search engine.
func (s *SearchEngineService) Create(ctx context.Context, domain, name string) (*ent.SearchEngines, error) {
	engine, err := s.db.Client.SearchEngines.Create().
		SetDomain(domain).
		SetName(name).
		Save(ctx)
	if err == nil {
		sql.InvalidateSearchEngineCache()
	}
	return engine, err
}

// Update modifies an existing built-in search engine.
func (s *SearchEngineService) Update(ctx context.Context, id int64, domain, name string) (*ent.SearchEngines, error) {
	engine, err := s.db.Client.SearchEngines.UpdateOneID(id).
		SetDomain(domain).
		SetName(name).
		Save(ctx)
	if err == nil {
		sql.InvalidateSearchEngineCache()
	}
	return engine, err
}

// Delete removes a built-in search engine.
func (s *SearchEngineService) Delete(ctx context.Context, id int64) error {
	if err := s.db.Client.SearchEngines.DeleteOneID(id).Exec(ctx); err != nil {
		return err
	}
	sql.InvalidateSearchEngineCache()
	return nil
}
