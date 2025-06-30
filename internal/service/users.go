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

type UserService struct {
	db *postgresql.Client
}

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

func (s *UserService) GetUserByEmail(ctx context.Context, email string) (*ent.User, error) {
	return s.db.Client.User.Query().Where(user.Email(email)).Only(ctx)
}

func (s *UserService) GetUserByID(ctx context.Context, id int64) (*ent.User, error) {
	return s.db.Client.User.Query().Where(user.ID(id)).Only(ctx)
}

func (s *UserService) CheckPassword(ctx context.Context, user *ent.User, password string) bool {
	return utils.CheckBcrypt(password, user.PasswordHash)
}
