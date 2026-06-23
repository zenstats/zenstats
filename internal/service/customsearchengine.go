package service

import (
	"context"
	"sync"

	"github.com/zenstats/zenstats/internal/service/stats/sql"
	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/customsearchengine"
	"github.com/zenstats/zenstats/pkg/globals"
)

// CustomSearchEngineService 自定义搜索引擎服务，提供自定义搜索引擎的CRUD操作。
type CustomSearchEngineService struct {
	db *postgresql.Client
}

// GetCustomSearchEngineService 获取 CustomSearchEngineService 单例实例。
var GetCustomSearchEngineService = sync.OnceValue(func() *CustomSearchEngineService {
	db := globals.GetDB()
	if db == nil {
		panic("DB is not initialized")
	}
	return &CustomSearchEngineService{db: db}
})

// GetUserSearchEngines 获取用户的自定义搜索引擎列表
func (s *CustomSearchEngineService) GetUserSearchEngines(ctx context.Context, userID int64) ([]*ent.CustomSearchEngine, error) {
	return s.db.Client.CustomSearchEngine.Query().
		Where(customsearchengine.UserID(userID)).
		All(ctx)
}

// GetSearchEngineByID 根据ID获取自定义搜索引擎
func (s *CustomSearchEngineService) GetSearchEngineByID(ctx context.Context, id int64) (*ent.CustomSearchEngine, error) {
	return s.db.Client.CustomSearchEngine.Get(ctx, id)
}

// CreateSearchEngine 创建自定义搜索引擎
func (s *CustomSearchEngineService) CreateSearchEngine(ctx context.Context, userID int64, domain, name string) (*ent.CustomSearchEngine, error) {
	engine, err := s.db.Client.CustomSearchEngine.Create().
		SetUserID(userID).
		SetDomain(domain).
		SetName(name).
		Save(ctx)
	if err == nil {
		sql.InvalidateUserSearchEngineCache(userID)
	}
	return engine, err
}

// UpdateSearchEngine 更新自定义搜索引擎
func (s *CustomSearchEngineService) UpdateSearchEngine(ctx context.Context, id int64, domain, name string) (*ent.CustomSearchEngine, error) {
	engine, err := s.db.Client.CustomSearchEngine.UpdateOneID(id).
		SetDomain(domain).
		SetName(name).
		Save(ctx)
	if err == nil && engine != nil {
		sql.InvalidateUserSearchEngineCache(engine.UserID)
	}
	return engine, err
}

// DeleteSearchEngine 删除自定义搜索引擎
func (s *CustomSearchEngineService) DeleteSearchEngine(ctx context.Context, id int64) error {
	engine, err := s.db.Client.CustomSearchEngine.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.db.Client.CustomSearchEngine.DeleteOneID(id).Exec(ctx); err != nil {
		return err
	}
	sql.InvalidateUserSearchEngineCache(engine.UserID)
	return nil
}

// GetUserSearchEngineCount 获取用户自定义搜索引擎数量
func (s *CustomSearchEngineService) GetUserSearchEngineCount(ctx context.Context, userID int64) (int, error) {
	return s.db.Client.CustomSearchEngine.Query().
		Where(customsearchengine.UserID(userID)).
		Count(ctx)
}

// HasSearchEnginePermission 检查用户是否有自定义搜索引擎权限
func (s *CustomSearchEngineService) HasSearchEnginePermission(ctx context.Context, userID int64) (bool, error) {
	userService := GetUserService()
	user, err := userService.GetUserWithConfig(ctx, userID)
	if err != nil {
		return false, err
	}

	if user.Edges.UserConfig == nil || user.Edges.UserConfig.Edges.Group == nil {
		return false, nil
	}

	return user.Edges.UserConfig.Edges.Group.CustomSearchEngines, nil
}
