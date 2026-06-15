package service

import (
	"context"
	"sync"
	"time"

	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/subaccount"
	"github.com/zenstats/zenstats/pkg/globals"
	"github.com/zenstats/zenstats/pkg/utils"
)

var (
	subAccountServiceInstance *SubAccountService
	subAccountOnce            sync.Once
)

// SubAccountService 子账号服务，提供子账号的CRUD操作。
type SubAccountService struct {
	db *postgresql.Client
}

// GetSubAccountService 获取 SubAccountService 单例实例。
func GetSubAccountService() *SubAccountService {
	subAccountOnce.Do(func() {
		db := globals.GetDB()
		if db == nil {
			panic("DB is not initialized")
		}
		subAccountServiceInstance = &SubAccountService{db: db}
	})
	return subAccountServiceInstance
}

// GetUserSubAccounts 获取用户的子账号列表
func (s *SubAccountService) GetUserSubAccounts(ctx context.Context, userID int64) ([]*ent.SubAccount, error) {
	return s.db.Client.SubAccount.Query().
		Where(subaccount.ParentUserID(userID)).
		All(ctx)
}

// GetSubAccountByID 根据ID获取子账号
func (s *SubAccountService) GetSubAccountByID(ctx context.Context, id int64) (*ent.SubAccount, error) {
	return s.db.Client.SubAccount.Get(ctx, id)
}

// CreateSubAccount 创建子账号
func (s *SubAccountService) CreateSubAccount(ctx context.Context, parentUserID int64, email, name, password string) (*ent.SubAccount, error) {
	passwordHash, err := utils.GeneratedBcrypt(password)
	if err != nil {
		return nil, err
	}

	return s.db.Client.SubAccount.Create().
		SetParentUserID(parentUserID).
		SetEmail(email).
		SetName(name).
		SetPasswordHash(passwordHash).
		SetRole("viewer").
		SetStatus("active").
		Save(ctx)
}

// UpdateSubAccount 更新子账号
func (s *SubAccountService) UpdateSubAccount(ctx context.Context, id int64, name, status string) (*ent.SubAccount, error) {
	update := s.db.Client.SubAccount.UpdateOneID(id)
	if name != "" {
		update = update.SetName(name)
	}
	if status != "" {
		update = update.SetStatus(status)
	}
	return update.Save(ctx)
}

// DeleteSubAccount 删除子账号
func (s *SubAccountService) DeleteSubAccount(ctx context.Context, id int64) error {
	return s.db.Client.SubAccount.DeleteOneID(id).Exec(ctx)
}

// ResetSubAccountPassword 重置子账号密码
func (s *SubAccountService) ResetSubAccountPassword(ctx context.Context, id int64, newPassword string) error {
	passwordHash, err := utils.GeneratedBcrypt(newPassword)
	if err != nil {
		return err
	}

	return s.db.Client.SubAccount.UpdateOneID(id).
		SetPasswordHash(passwordHash).
		Exec(ctx)
}

// GetUserSubAccountCount 获取用户子账号数量
func (s *SubAccountService) GetUserSubAccountCount(ctx context.Context, userID int64) (int, error) {
	return s.db.Client.SubAccount.Query().
		Where(subaccount.ParentUserID(userID)).
		Count(ctx)
}

// HasSubAccountPermission 检查用户是否有子账号权限
func (s *SubAccountService) HasSubAccountPermission(ctx context.Context, userID int64) (bool, error) {
	userService := GetUserService()
	user, err := userService.GetUserWithConfig(ctx, userID)
	if err != nil {
		return false, err
	}

	if user.Edges.UserConfig == nil || user.Edges.UserConfig.Edges.Group == nil {
		return false, nil
	}

	return user.Edges.UserConfig.Edges.Group.MaxSubAccounts != 0, nil
}

// GetMaxSubAccounts 获取用户最大子账号数量
func (s *SubAccountService) GetMaxSubAccounts(ctx context.Context, userID int64) (int, error) {
	userService := GetUserService()
	user, err := userService.GetUserWithConfig(ctx, userID)
	if err != nil {
		return 0, err
	}

	if user.Edges.UserConfig == nil || user.Edges.UserConfig.Edges.Group == nil {
		return 0, nil
	}

	return user.Edges.UserConfig.Edges.Group.MaxSubAccounts, nil
}

// SubAccountLogin 子账号登录
func (s *SubAccountService) SubAccountLogin(ctx context.Context, email, password string) (*ent.SubAccount, error) {
	subAccount, err := s.db.Client.SubAccount.Query().
		Where(subaccount.Email(email)).
		Only(ctx)
	if err != nil {
		return nil, err
	}

	if subAccount.Status != "active" {
		return nil, ErrSubAccountSuspended
	}

	if !utils.CheckBcrypt(password, subAccount.PasswordHash) {
		return nil, ErrInvalidPassword
	}

	// 更新最后登录时间
	_ = s.db.Client.SubAccount.UpdateOneID(subAccount.ID).
		SetLastSeen(time.Now()).
		Exec(ctx)

	return subAccount, nil
}

// 错误定义
var (
	ErrSubAccountSuspended = &SubAccountSuspendedError{}
	ErrInvalidPassword     = &InvalidPasswordError{}
)

type SubAccountSuspendedError struct{}

func (e *SubAccountSuspendedError) Error() string {
	return "sub account is suspended"
}

type InvalidPasswordError struct{}

func (e *InvalidPasswordError) Error() string {
	return "invalid password"
}
