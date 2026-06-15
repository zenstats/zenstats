package service

import (
	"context"
	"sync"

	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/customsearchengine"
	"github.com/zenstats/zenstats/pkg/globals"
)

var (
	customSearchEngineServiceInstance *CustomSearchEngineService
	customSearchEngineOnce            sync.Once
)

// CustomSearchEngineService 自定义搜索引擎服务，提供自定义搜索引擎的CRUD操作。
type CustomSearchEngineService struct {
	db *postgresql.Client
}

// GetCustomSearchEngineService 获取 CustomSearchEngineService 单例实例。
func GetCustomSearchEngineService() *CustomSearchEngineService {
	customSearchEngineOnce.Do(func() {
		db := globals.GetDB()
		if db == nil {
			panic("DB is not initialized")
		}
		customSearchEngineServiceInstance = &CustomSearchEngineService{db: db}
	})
	return customSearchEngineServiceInstance
}

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
	return s.db.Client.CustomSearchEngine.Create().
		SetUserID(userID).
		SetDomain(domain).
		SetName(name).
		Save(ctx)
}

// UpdateSearchEngine 更新自定义搜索引擎
func (s *CustomSearchEngineService) UpdateSearchEngine(ctx context.Context, id int64, domain, name string) (*ent.CustomSearchEngine, error) {
	return s.db.Client.CustomSearchEngine.UpdateOneID(id).
		SetDomain(domain).
		SetName(name).
		Save(ctx)
}

// DeleteSearchEngine 删除自定义搜索引擎
func (s *CustomSearchEngineService) DeleteSearchEngine(ctx context.Context, id int64) error {
	return s.db.Client.CustomSearchEngine.DeleteOneID(id).Exec(ctx)
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
