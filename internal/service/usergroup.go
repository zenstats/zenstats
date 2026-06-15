package service

import (
	"context"
	"sync"

	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/userconfig"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/usergroup"
	"github.com/zenstats/zenstats/pkg/globals"
)

var (
	userGroupServiceInstance *UserGroupService
	userGroupOnce            sync.Once
)

// UserGroupService 用户组服务，提供用户组/套餐相关的数据库操作。
type UserGroupService struct {
	db *postgresql.Client
}

// GetUserGroupService 获取 UserGroupService 单例实例。
func GetUserGroupService() *UserGroupService {
	userGroupOnce.Do(func() {
		db := globals.GetDB()
		if db == nil {
			panic("DB is not initialized")
		}
		userGroupServiceInstance = &UserGroupService{db: db}
	})
	return userGroupServiceInstance
}

// GetAllGroups 获取所有用户组
func (s *UserGroupService) GetAllGroups(ctx context.Context) ([]*ent.UserGroup, error) {
	return s.db.Client.UserGroup.Query().All(ctx)
}

// GetGroupByID 根据ID获取用户组
func (s *UserGroupService) GetGroupByID(ctx context.Context, id int64) (*ent.UserGroup, error) {
	return s.db.Client.UserGroup.Get(ctx, id)
}

// CreateGroup 创建用户组
func (s *UserGroupService) CreateGroup(ctx context.Context, name, description string, maxSites, maxMonthlyEvents, maxAPIKeys, maxSubAccounts int, customSearchEngines, isDefault bool, price float64) (*ent.UserGroup, error) {
	// 如果设置为默认套餐，先取消其他默认套餐
	if isDefault {
		_, err := s.db.Client.UserGroup.Update().
			Where(usergroup.IsDefault(true)).
			SetIsDefault(false).
			Save(ctx)
		if err != nil {
			return nil, err
		}
	}

	return s.db.Client.UserGroup.Create().
		SetName(name).
		SetDescription(description).
		SetMaxSites(maxSites).
		SetMaxMonthlyEvents(maxMonthlyEvents).
		SetMaxAPIKeys(maxAPIKeys).
		SetMaxSubAccounts(maxSubAccounts).
		SetCustomSearchEngines(customSearchEngines).
		SetIsDefault(isDefault).
		SetPrice(price).
		Save(ctx)
}

// UpdateGroup 更新用户组
func (s *UserGroupService) UpdateGroup(ctx context.Context, id int64, name, description string, maxSites, maxMonthlyEvents, maxAPIKeys, maxSubAccounts int, customSearchEngines, isDefault bool, price float64) (*ent.UserGroup, error) {
	// 如果设置为默认套餐，先取消其他默认套餐
	if isDefault {
		_, err := s.db.Client.UserGroup.Update().
			Where(usergroup.IsDefault(true)).
			SetIsDefault(false).
			Save(ctx)
		if err != nil {
			return nil, err
		}
	}

	return s.db.Client.UserGroup.UpdateOneID(id).
		SetName(name).
		SetDescription(description).
		SetMaxSites(maxSites).
		SetMaxMonthlyEvents(maxMonthlyEvents).
		SetMaxAPIKeys(maxAPIKeys).
		SetMaxSubAccounts(maxSubAccounts).
		SetCustomSearchEngines(customSearchEngines).
		SetIsDefault(isDefault).
		SetPrice(price).
		Save(ctx)
}

// DeleteGroup 删除用户组
func (s *UserGroupService) DeleteGroup(ctx context.Context, id int64) error {
	// 检查是否有用户使用此套餐
	count, err := s.db.Client.UserConfig.Query().
		Where(userconfig.GroupID(id)).
		Count(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrGroupHasUsers
	}

	return s.db.Client.UserGroup.DeleteOneID(id).Exec(ctx)
}

// GetGroupUserCount 获取用户组的用户数量
func (s *UserGroupService) GetGroupUserCount(ctx context.Context, groupID int64) (int, error) {
	return s.db.Client.UserConfig.Query().
		Where(userconfig.GroupID(groupID)).
		Count(ctx)
}

// GetDefaultGroup 获取默认用户组
func (s *UserGroupService) GetDefaultGroup(ctx context.Context) (*ent.UserGroup, error) {
	return s.db.Client.UserGroup.Query().
		Where(usergroup.IsDefault(true)).
		Only(ctx)
}

// SetDefaultGroup 设置默认用户组
func (s *UserGroupService) SetDefaultGroup(ctx context.Context, id int64) error {
	// 先取消其他默认套餐
	_, err := s.db.Client.UserGroup.Update().
		Where(usergroup.IsDefault(true)).
		SetIsDefault(false).
		Save(ctx)
	if err != nil {
		return err
	}

	return s.db.Client.UserGroup.UpdateOneID(id).SetIsDefault(true).Exec(ctx)
}

// 错误定义
var ErrGroupHasUsers = &GroupHasUsersError{}

type GroupHasUsersError struct{}

func (e *GroupHasUsersError) Error() string {
	return "cannot delete group with assigned users"
}
