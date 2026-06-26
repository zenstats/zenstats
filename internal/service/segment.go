package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	entsegment "github.com/zenstats/zenstats/internal/store/postgresql/ent/segment"
	"github.com/zenstats/zenstats/pkg/globals"
)

// Segment 已保存的过滤器组合。
type Segment struct {
	ID          int64     `json:"id"`
	SiteID      int64     `json:"site_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Filters     string    `json:"filters"`
	CreatedBy   *int64    `json:"created_by,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SegmentService Segment 业务逻辑。
type SegmentService struct {
	db *postgresql.Client
}

// GetSegmentService 获取 SegmentService 单例实例。
var GetSegmentService = sync.OnceValue(func() *SegmentService {
	db := globals.GetDB()
	if db == nil {
		panic("DB is not initialized")
	}
	return &SegmentService{db: db}
})

func toSegment(e *ent.Segment) *Segment {
	return &Segment{
		ID:          e.ID,
		SiteID:      e.SiteID,
		Name:        e.Name,
		Description: e.Description,
		Filters:     e.Filters,
		CreatedBy:   e.CreatedBy,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}

// List 列出站点下所有 Segment。
func (s *SegmentService) List(ctx context.Context, siteID int64) ([]*Segment, error) {
	rows, err := s.db.Client.Segment.
		Query().
		Where(entsegment.SiteID(siteID)).
		Order(ent.Desc(entsegment.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list segments: %w", err)
	}
	out := make([]*Segment, len(rows))
	for i, r := range rows {
		out[i] = toSegment(r)
	}
	return out, nil
}

// Create 创建新 Segment。
func (s *SegmentService) Create(ctx context.Context, siteID, userID int64, name, description, filters string) (*Segment, error) {
	row, err := s.db.Client.Segment.
		Create().
		SetSiteID(siteID).
		SetName(name).
		SetNillableDescription(nilIfEmpty(description)).
		SetFilters(filters).
		SetCreatedBy(userID).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create segment: %w", err)
	}
	return toSegment(row), nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Update 更新 Segment（name / description / filters，空字符串表示不更新）。
func (s *SegmentService) Update(ctx context.Context, siteID, segID int64, name, description, filters string) (*Segment, error) {
	q := s.db.Client.Segment.
		UpdateOneID(segID).
		Where(entsegment.SiteID(siteID))
	if name != "" {
		q = q.SetName(name)
	}
	if description != "" {
		q = q.SetDescription(description)
	}
	if filters != "" {
		q = q.SetFilters(filters)
	}
	row, err := q.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("update segment: %w", err)
	}
	return toSegment(row), nil
}

// Delete 删除 Segment（校验 site_id 归属）。
func (s *SegmentService) Delete(ctx context.Context, siteID, segID int64) error {
	n, err := s.db.Client.Segment.
		Delete().
		Where(
			entsegment.ID(segID),
			entsegment.SiteID(siteID),
		).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete segment: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("segment not found")
	}
	return nil
}
