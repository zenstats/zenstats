package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	entsavedpanel "github.com/zenstats/zenstats/internal/store/postgresql/ent/savedpanel"
	"github.com/zenstats/zenstats/pkg/globals"
)

// SavedPanel 已保存的自定义查询面板（含完整查询配置）。
type SavedPanel struct {
	ID          int64     `json:"id"`
	SiteID      int64     `json:"site_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Config      string    `json:"config"`
	Pinned      bool      `json:"pinned"`
	Layout      string    `json:"layout"`
	CreatedBy   *int64    `json:"created_by,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SavedPanelService SavedPanel 业务逻辑。
type SavedPanelService struct {
	db *postgresql.Client
}

// GetSavedPanelService 获取 SavedPanelService 单例。
var GetSavedPanelService = sync.OnceValue(func() *SavedPanelService {
	db := globals.GetDB()
	if db == nil {
		panic("DB is not initialized")
	}
	return &SavedPanelService{db: db}
})

func toSavedPanel(e *ent.SavedPanel) *SavedPanel {
	return &SavedPanel{
		ID:          e.ID,
		SiteID:      e.SiteID,
		Name:        e.Name,
		Description: e.Description,
		Config:      e.Config,
		Pinned:      e.Pinned,
		Layout:      e.Layout,
		CreatedBy:   e.CreatedBy,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}

// List 列出站点下所有面板。
func (s *SavedPanelService) List(ctx context.Context, siteID int64) ([]*SavedPanel, error) {
	rows, err := s.db.Client.SavedPanel.
		Query().
		Where(entsavedpanel.SiteID(siteID)).
		Order(ent.Desc(entsavedpanel.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list saved panels: %w", err)
	}
	out := make([]*SavedPanel, len(rows))
	for i, r := range rows {
		out[i] = toSavedPanel(r)
	}
	return out, nil
}

// ListPinned 列出站点下固定到首页的面板。
func (s *SavedPanelService) ListPinned(ctx context.Context, siteID int64) ([]*SavedPanel, error) {
	rows, err := s.db.Client.SavedPanel.
		Query().
		Where(
			entsavedpanel.SiteID(siteID),
			entsavedpanel.Pinned(true),
		).
		Order(ent.Desc(entsavedpanel.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list pinned panels: %w", err)
	}
	out := make([]*SavedPanel, len(rows))
	for i, r := range rows {
		out[i] = toSavedPanel(r)
	}
	return out, nil
}

// Create 创建新面板。
func (s *SavedPanelService) Create(ctx context.Context, siteID, userID int64, name, description, config string, pinned bool) (*SavedPanel, error) {
	row, err := s.db.Client.SavedPanel.
		Create().
		SetSiteID(siteID).
		SetName(name).
		SetNillableDescription(nilIfEmpty(description)).
		SetConfig(config).
		SetPinned(pinned).
		SetCreatedBy(userID).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create saved panel: %w", err)
	}
	return toSavedPanel(row), nil
}

// Update 更新面板。name / config / layout 空字符串表示不更新；
// description 用指针区分：nil=不更新，""=清空，其它=更新；pinned 通过指针可选更新。
func (s *SavedPanelService) Update(ctx context.Context, siteID, panelID int64, name string, description *string, config, layout string, pinned *bool) (*SavedPanel, error) {
	q := s.db.Client.SavedPanel.
		UpdateOneID(panelID).
		Where(entsavedpanel.SiteID(siteID))
	if name != "" {
		q = q.SetName(name)
	}
	if description != nil {
		if *description == "" {
			q = q.ClearDescription()
		} else {
			q = q.SetDescription(*description)
		}
	}
	if config != "" {
		q = q.SetConfig(config)
	}
	if layout != "" {
		q = q.SetLayout(layout)
	}
	if pinned != nil {
		q = q.SetPinned(*pinned)
	}
	row, err := q.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("update saved panel: %w", err)
	}
	return toSavedPanel(row), nil
}

// Delete 删除面板（校验 site_id 归属）。
func (s *SavedPanelService) Delete(ctx context.Context, siteID, panelID int64) error {
	n, err := s.db.Client.SavedPanel.
		Delete().
		Where(
			entsavedpanel.ID(panelID),
			entsavedpanel.SiteID(siteID),
		).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete saved panel: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("saved panel not found")
	}
	return nil
}
