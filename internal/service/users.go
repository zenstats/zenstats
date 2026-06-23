package service

import (
	"context"
	"sync"

	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/user"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/userconfig"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/usergroup"
	"github.com/zenstats/zenstats/pkg/bcrypt"
	"github.com/zenstats/zenstats/pkg/globals"
)

// UserService 用户服务，提供用户相关的数据库操作。
type UserService struct {
	db *postgresql.Client
}

// GetUserService 获取 UserService 单例实例。
var GetUserService = sync.OnceValue(func() *UserService {
	db := globals.GetDB()
	if db == nil {
		panic("DB is not initialized")
	}
	return &UserService{db: db}
})

// GetUserCount 获取系统中的用户总数。
func (s *UserService) GetUserCount(ctx context.Context) (int, error) {
	return s.db.Client.User.Query().Count(ctx)
}

// CreateUser 创建新用户，密码会通过 bcrypt 加密存储。
func (s *UserService) CreateUser(ctx context.Context, name, email, password string) (*ent.User, error) {

	passwordHash, err := bcrypt.Generate(password)
	if err != nil {
		return nil, err
	}
	return s.db.Client.User.Create().
		SetEmail(email).
		SetName(name).
		SetPasswordHash(passwordHash).
		Save(ctx)
}

// GetUserByEmail 根据邮箱地址查询用户。
func (s *UserService) GetUserByEmail(ctx context.Context, email string) (*ent.User, error) {
	return s.db.Client.User.Query().Where(user.Email(email)).Only(ctx)
}

// GetUserByID 根据用户 ID 查询用户。
func (s *UserService) GetUserByID(ctx context.Context, id int64) (*ent.User, error) {
	return s.db.Client.User.Query().Where(user.ID(id)).Only(ctx)
}

// CheckPassword 验证用户密码是否正确。
func (s *UserService) CheckPassword(ctx context.Context, user *ent.User, password string) bool {
	return bcrypt.Check(password, user.PasswordHash)
}

// UpdatePassword 更新用户密码
func (s *UserService) UpdatePassword(ctx context.Context, userID int64, newPassword string) error {
	passwordHash, err := bcrypt.Generate(newPassword)
	if err != nil {
		return err
	}
	return s.db.Client.User.UpdateOneID(userID).
		SetPasswordHash(passwordHash).
		Exec(ctx)
}

// CreateUserConfig 为新用户创建默认配置
func (s *UserService) CreateUserConfig(ctx context.Context, userID int64) error {
	// 获取默认套餐
	defaultGroup, err := s.db.Client.UserGroup.Query().
		Where(usergroup.IsDefault(true)).
		Only(ctx)
	if err != nil {
		// 如果没有默认套餐，使用第一个套餐
		defaultGroup, err = s.db.Client.UserGroup.Query().First(ctx)
		if err != nil {
			return err
		}
	}

	// 创建用户配置
	_, err = s.db.Client.UserConfig.Create().
		SetUserID(userID).
		SetGroupID(defaultGroup.ID).
		SetStatus("active").
		Save(ctx)
	return err
}

// IsAdmin 检查用户是否为管理员
func (s *UserService) IsAdmin(ctx context.Context, userID int64) (bool, error) {
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return false, err
	}
	return user.IsAdmin, nil
}

// SetAdmin 设置用户管理员状态
func (s *UserService) SetAdmin(ctx context.Context, userID int64, isAdmin bool) error {
	return s.db.Client.User.UpdateOneID(userID).SetIsAdmin(isAdmin).Exec(ctx)
}

// SetEmailVerified 设置用户邮箱验证状态
func (s *UserService) SetEmailVerified(ctx context.Context, userID int64, verified bool) error {
	return s.db.Client.User.UpdateOneID(userID).SetEmailVerified(verified).Exec(ctx)
}

// GetUserWithConfig 获取用户及其配置信息
func (s *UserService) GetUserWithConfig(ctx context.Context, userID int64) (*ent.User, error) {
	return s.db.Client.User.Query().
		Where(user.ID(userID)).
		WithUserConfig(func(query *ent.UserConfigQuery) {
			query.WithGroup()
		}).
		Only(ctx)
}

// GetAllUsers 获取所有用户（分页）
func (s *UserService) GetAllUsers(ctx context.Context, offset, limit int) ([]*ent.User, error) {
	return s.db.Client.User.Query().
		WithUserConfig(func(query *ent.UserConfigQuery) {
			query.WithGroup()
		}).
		Offset(offset).
		Limit(limit).
		All(ctx)
}

// GetUserCount 获取用户总数
func (s *UserService) GetUserCountAdmin(ctx context.Context) (int, error) {
	return s.db.Client.User.Query().Count(ctx)
}

// UpdateUserStatus 更新用户状态
func (s *UserService) UpdateUserStatus(ctx context.Context, userID int64, status string) error {
	config, err := s.db.Client.UserConfig.Query().
		Where(userconfig.UserID(userID)).
		Only(ctx)
	if err != nil {
		return err
	}
	return s.db.Client.UserConfig.UpdateOneID(config.ID).SetStatus(status).Exec(ctx)
}

// UpdateUserGroup 更新用户套餐
func (s *UserService) UpdateUserGroup(ctx context.Context, userID int64, groupID int64) error {
	config, err := s.db.Client.UserConfig.Query().
		Where(userconfig.UserID(userID)).
		Only(ctx)
	if err != nil {
		// 如果没有配置，创建一个新的
		_, err = s.db.Client.UserConfig.Create().
			SetUserID(userID).
			SetGroupID(groupID).
			SetStatus("active").
			Save(ctx)
		return err
	}
	return s.db.Client.UserConfig.UpdateOneID(config.ID).SetGroupID(groupID).Exec(ctx)
}

// UpdateUserName 更新用户名
func (s *UserService) UpdateUserName(ctx context.Context, userID int64, name string) error {
	return s.db.Client.User.UpdateOneID(userID).SetName(name).Exec(ctx)
}
