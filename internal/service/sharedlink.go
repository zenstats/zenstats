package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zenstats/zenstats/pkg/bcrypt"

	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	entsharedlink "github.com/zenstats/zenstats/internal/store/postgresql/ent/sharedlink"
	"github.com/zenstats/zenstats/pkg/globals"
)

// SharedLink 共享链接数据结构。
type SharedLink struct {
	ID           int64     `json:"id"`
	SiteID       int64     `json:"site_id"`
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	PasswordHash string    `json:"password_hash,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// SharedLinkService 共享链接业务逻辑。
type SharedLinkService struct {
	db *postgresql.Client
}

// GetSharedLinkService 获取 SharedLinkService 单例实例。
var GetSharedLinkService = sync.OnceValue(func() *SharedLinkService {
	db := globals.GetDB()
	if db == nil {
		panic("DB is not initialized")
	}
	return &SharedLinkService{db: db}
})

func toSharedLink(e *ent.SharedLink) *SharedLink {
	return &SharedLink{
		ID:           e.ID,
		SiteID:       e.SiteID,
		Name:         e.Name,
		Slug:         e.Slug,
		PasswordHash: e.PasswordHash,
		CreatedAt:    e.CreatedAt,
		UpdatedAt:    e.UpdatedAt,
	}
}

// List 返回站点的所有共享链接（按创建时间倒序）。
func (s *SharedLinkService) List(ctx context.Context, siteID int64) ([]*SharedLink, error) {
	rows, err := s.db.Client.SharedLink.
		Query().
		Where(entsharedlink.SiteID(siteID)).
		Order(ent.Desc(entsharedlink.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list shared_links: %w", err)
	}
	out := make([]*SharedLink, len(rows))
	for i, r := range rows {
		out[i] = toSharedLink(r)
	}
	return out, nil
}

// Create 创建新共享链接（slug 由调用方生成）。
func (s *SharedLinkService) Create(ctx context.Context, siteID int64, name, slug, password string) (*SharedLink, error) {
	q := s.db.Client.SharedLink.
		Create().
		SetSiteID(siteID).
		SetName(name).
		SetSlug(slug)
	if password != "" {
		h, err := bcrypt.Generate(password)
		if err != nil {
			return nil, fmt.Errorf("hashing password: %w", err)
		}
		q = q.SetPasswordHash(h)
	}
	row, err := q.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create shared_link: %w", err)
	}
	return toSharedLink(row), nil
}

// GetBySlug 通过 slug 查询共享链接（公开访问，无需认证）。
func (s *SharedLinkService) GetBySlug(ctx context.Context, slug string) (*SharedLink, error) {
	row, err := s.db.Client.SharedLink.
		Query().
		Where(entsharedlink.Slug(slug)).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("shared link not found")
	}
	return toSharedLink(row), nil
}

// Delete 删除共享链接（校验 site_id 归属）。
func (s *SharedLinkService) Delete(ctx context.Context, siteID, linkID int64) error {
	n, err := s.db.Client.SharedLink.
		Delete().
		Where(
			entsharedlink.ID(linkID),
			entsharedlink.SiteID(siteID),
		).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete shared_link: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("shared link not found")
	}
	return nil
}
