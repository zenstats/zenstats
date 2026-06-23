package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/apikey"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/user"
	"github.com/zenstats/zenstats/pkg/globals"
)

// APIKeyService API Key 管理服务，提供 API Key 的创建、查询、删除和鉴权。
type APIKeyService struct {
	db *postgresql.Client
}

// GetAPIKeyService 获取 APIKeyService 单例实例。
var GetAPIKeyService = sync.OnceValue(func() *APIKeyService {
	db := globals.GetDB()
	if db == nil {
		panic("DB is not initialized")
	}
	return &APIKeyService{db: db}
})

// APIKeyInfo API Key 信息（不包含 key_hash）。
type APIKeyInfo struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Key        string `json:"key,omitempty"` // 仅创建时返回明文 key
	CreatedAt  string `json:"created_at"`
	LastUsedAt string `json:"last_used_at,omitempty"`
	ExpiresAt  string `json:"expires_at,omitempty"`
}

// CreateAPIKey 为指定用户创建 API Key，返回明文 key（仅此一次可见）。
// expiresAt 可选，为 nil 时永不过期。
func (s *APIKeyService) CreateAPIKey(ctx context.Context, userID int64, name string, expiresAt *time.Time) (*APIKeyInfo, string, error) {
	// 生成随机 key
	rawKey, err := generateRandomKey()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate key: %w", err)
	}

	// 计算 key hash
	keyHash := hashKey(rawKey)

	// 存储到数据库
	createQuery := s.db.Client.APIKey.Create().
		SetUserID(userID).
		SetName(name).
		SetKeyHash(keyHash)
	if expiresAt != nil {
		createQuery = createQuery.SetNillableExpiresAt(expiresAt)
	}
	apiKeyEntity, err := createQuery.Save(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create api key: %w", err)
	}

	info := &APIKeyInfo{
		ID:        apiKeyEntity.ID,
		Name:      apiKeyEntity.Name,
		CreatedAt: apiKeyEntity.CreatedAt.Format("2006-01-02 15:04:05"),
	}
	if apiKeyEntity.ExpiresAt != nil {
		info.ExpiresAt = apiKeyEntity.ExpiresAt.Format("2006-01-02 15:04:05")
	}
	return info, rawKey, nil
}

// ListAPIKeys 获取指定用户的所有 API Key 列表。
func (s *APIKeyService) ListAPIKeys(ctx context.Context, userID int64) ([]*APIKeyInfo, error) {
	keys, err := s.db.Client.APIKey.Query().
		Where(apikey.UserID(userID)).
		Order(ent.Desc(apikey.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list api keys: %w", err)
	}

	result := make([]*APIKeyInfo, len(keys))
	for i, k := range keys {
		info := &APIKeyInfo{
			ID:        k.ID,
			Name:      k.Name,
			CreatedAt: k.CreatedAt.Format("2006-01-02 15:04:05"),
		}
		if k.LastUsedAt != nil {
			info.LastUsedAt = k.LastUsedAt.Format("2006-01-02 15:04:05")
		}
		if k.ExpiresAt != nil {
			info.ExpiresAt = k.ExpiresAt.Format("2006-01-02 15:04:05")
		}
		result[i] = info
	}
	return result, nil
}

// DeleteAPIKey 删除指定用户的 API Key。
func (s *APIKeyService) DeleteAPIKey(ctx context.Context, userID int64, keyID int64) error {
	count, err := s.db.Client.APIKey.Query().
		Where(
			apikey.ID(keyID),
			apikey.UserID(userID),
		).
		Count(ctx)
	if err != nil {
		return fmt.Errorf("failed to query api key: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("api key not found")
	}

	return s.db.Client.APIKey.DeleteOneID(keyID).Exec(ctx)
}

// ValidateAPIKey 验证 API Key，返回关联的用户 ID。
// 同时检查过期时间并更新最后使用时间。
func (s *APIKeyService) ValidateAPIKey(ctx context.Context, rawKey string) (int64, error) {
	keyHash := hashKey(rawKey)

	apiKeyEntity, err := s.db.Client.APIKey.Query().
		Where(apikey.KeyHash(keyHash)).
		WithUser().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return 0, fmt.Errorf("invalid api key")
		}
		return 0, fmt.Errorf("failed to validate api key: %w", err)
	}

	// 检查是否过期
	if apiKeyEntity.ExpiresAt != nil && time.Now().After(*apiKeyEntity.ExpiresAt) {
		return 0, fmt.Errorf("api key has expired")
	}

	// 更新最后使用时间
	now := time.Now()
	_ = s.db.Client.APIKey.UpdateOne(apiKeyEntity).
		SetNillableLastUsedAt(&now).
		Exec(ctx)

	return apiKeyEntity.UserID, nil
}

// GetUserAPIKeyCount 获取用户 API Key 数量
func (s *APIKeyService) GetUserAPIKeyCount(ctx context.Context, userID int64) (int, error) {
	return s.db.Client.APIKey.Query().
		Where(apikey.UserID(userID)).
		Count(ctx)
}

// GetAPIKeyByHash 通过 key_hash 获取 API Key 及关联的用户信息。
func (s *APIKeyService) GetAPIKeyByHash(ctx context.Context, keyHash string) (*ent.APIKey, error) {
	return s.db.Client.APIKey.Query().
		Where(apikey.KeyHash(keyHash)).
		WithUser(func(uq *ent.UserQuery) {
			uq.Select(user.FieldID, user.FieldName, user.FieldEmail)
		}).
		Only(ctx)
}

// GetAPIKeyUser 通过 API Key 获取用户信息（含站点成员关系）。
func (s *APIKeyService) GetAPIKeyUser(ctx context.Context, rawKey string) (*ent.User, error) {
	keyHash := hashKey(rawKey)

	apiKeyEntity, err := s.db.Client.APIKey.Query().
		Where(apikey.KeyHash(keyHash)).
		WithUser().
		Only(ctx)
	if err != nil {
		return nil, err
	}

	return apiKeyEntity.Edges.User, nil
}

// generateRandomKey 生成 32 字节的随机 key，以 hex 编码返回。
func generateRandomKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "zen_" + hex.EncodeToString(bytes), nil
}

// hashKey 计算 key 的 SHA-256 哈希。
func hashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}
