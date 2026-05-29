package service

import (
	"context"
	"sync"

	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/user"
	"github.com/zenstats/zenstats/pkg/globals"
	"github.com/zenstats/zenstats/pkg/utils"
)

var (
	userServiceInstance *UserService
	userOnce            sync.Once
)

// UserService 用户服务，提供用户相关的数据库操作。
type UserService struct {
	db *postgresql.Client
}

// GetUserService 获取 UserService 单例实例。
func GetUserService() *UserService {
	userOnce.Do(func() {
		db := globals.GetDB()
		if db == nil {
			panic("DB is not initialized")
		}
		userServiceInstance = &UserService{db: db}
	})
	return userServiceInstance
}

// GetUserCount 获取系统中的用户总数。
func (s *UserService) GetUserCount(ctx context.Context) (int, error) {
	return s.db.Client.User.Query().Count(ctx)
}

// CreateUser 创建新用户，密码会通过 bcrypt 加密存储。
func (s *UserService) CreateUser(ctx context.Context, name, email, password string) (*ent.User, error) {

	passwordHash, err := utils.GeneratedBcrypt(password)
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
	return utils.CheckBcrypt(password, user.PasswordHash)
}
