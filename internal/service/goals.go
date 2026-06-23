package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/funnelstep"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/goal"
	"github.com/zenstats/zenstats/pkg/globals"
)

// GoalService 目标管理服务，提供目标的增删改查功能。
type GoalService struct {
	db *postgresql.Client
}

// GetGoalService 获取 GoalService 单例实例。
var GetGoalService = sync.OnceValue(func() *GoalService {
	db := globals.GetDB()
	if db == nil {
		panic("DB is not initialized")
	}
	return &GoalService{db: db}
})

// Goal 目标数据传输对象。
type Goal struct {
	ID          int64             `json:"id"`
	SiteID      int64             `json:"site_id"`
	EventName   string            `json:"event_name,omitempty"`
	PagePath    string            `json:"page_path,omitempty"`
	DisplayName string            `json:"display_name"`
	CustomProps map[string]string `json:"custom_props,omitempty"`
}

// CreateGoalRequest 创建目标请求参数。
type CreateGoalRequest struct {
	EventName   string            `json:"event_name" binding:"omitempty"`
	PagePath    string            `json:"page_path" binding:"omitempty"`
	DisplayName string            `json:"display_name" binding:"required"`
	CustomProps map[string]string `json:"custom_props,omitempty"`
}

// UpdateGoalRequest 更新目标请求参数。
type UpdateGoalRequest struct {
	DisplayName string            `json:"display_name" binding:"omitempty"`
	CustomProps map[string]string `json:"custom_props,omitempty"`
}

// ListGoals 获取站点的所有目标。
func (s *GoalService) ListGoals(ctx context.Context, siteID int64) ([]*Goal, error) {
	goals, err := s.db.Client.Goal.Query().
		Where(goal.SiteID(siteID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list goals: %w", err)
	}

	result := make([]*Goal, len(goals))
	for i, g := range goals {
		result[i] = &Goal{
			ID:          g.ID,
			SiteID:      g.SiteID,
			EventName:   g.EventName,
			PagePath:    g.PagePath,
			DisplayName: g.DisplayName,
			CustomProps: g.CustomProps,
		}
	}
	return result, nil
}

// GetGoal 获取单个目标。
func (s *GoalService) GetGoal(ctx context.Context, siteID int64, goalID int64) (*Goal, error) {
	g, err := s.db.Client.Goal.Query().
		Where(goal.ID(goalID), goal.SiteID(siteID)).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("goal not found")
	}

	return &Goal{
		ID:          g.ID,
		SiteID:      g.SiteID,
		EventName:   g.EventName,
		PagePath:    g.PagePath,
		DisplayName: g.DisplayName,
		CustomProps: g.CustomProps,
	}, nil
}

// CreateGoal 创建新目标。
func (s *GoalService) CreateGoal(ctx context.Context, siteID int64, req *CreateGoalRequest) (*Goal, error) {
	if req.EventName == "" && req.PagePath == "" {
		return nil, fmt.Errorf("either event_name or page_path must be provided")
	}
	if req.EventName != "" && req.PagePath != "" {
		return nil, fmt.Errorf("only one of event_name or page_path can be provided")
	}

	builder := s.db.Client.Goal.Create().
		SetSiteID(siteID).
		SetDisplayName(req.DisplayName)

	if req.EventName != "" {
		builder = builder.SetEventName(req.EventName)
	}
	if req.PagePath != "" {
		builder = builder.SetPagePath(req.PagePath)
	}
	if len(req.CustomProps) > 0 {
		builder = builder.SetCustomProps(req.CustomProps)
	}

	g, err := builder.Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, fmt.Errorf("goal with display name '%s' already exists", req.DisplayName)
		}
		return nil, fmt.Errorf("failed to create goal: %w", err)
	}

	return &Goal{
		ID:          g.ID,
		SiteID:      g.SiteID,
		EventName:   g.EventName,
		PagePath:    g.PagePath,
		DisplayName: g.DisplayName,
		CustomProps: g.CustomProps,
	}, nil
}

// UpdateGoal 更新目标。
func (s *GoalService) UpdateGoal(ctx context.Context, siteID int64, goalID int64, req *UpdateGoalRequest) (*Goal, error) {
	builder := s.db.Client.Goal.UpdateOneID(goalID).
		Where(goal.SiteID(siteID))

	if req.DisplayName != "" {
		builder = builder.SetDisplayName(req.DisplayName)
	}
	if req.CustomProps != nil {
		if len(req.CustomProps) > 0 {
			builder = builder.SetCustomProps(req.CustomProps)
		} else {
			builder = builder.ClearCustomProps()
		}
	}

	g, err := builder.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update goal: %w", err)
	}

	return &Goal{
		ID:          g.ID,
		SiteID:      g.SiteID,
		EventName:   g.EventName,
		PagePath:    g.PagePath,
		DisplayName: g.DisplayName,
		CustomProps: g.CustomProps,
	}, nil
}

// DeleteGoal 删除目标。
func (s *GoalService) DeleteGoal(ctx context.Context, siteID int64, goalID int64) error {
	// 检查目标是否被漏斗引用
	count, err := s.db.Client.FunnelStep.Query().
		Where(funnelstep.GoalID(goalID)).
		Count(ctx)
	if err != nil {
		return fmt.Errorf("failed to check goal usage: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("目标已被 %d 个漏斗使用，请先从漏斗中移除", count)
	}

	err = s.db.Client.Goal.DeleteOneID(goalID).
		Where(goal.SiteID(siteID)).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete goal: %w", err)
	}
	return nil
}

// GetGoalsForSite 获取站点的所有目标名称（用于统计查询验证）。
func (s *GoalService) GetGoalsForSite(ctx context.Context, siteID int64) ([]string, error) {
	goals, err := s.db.Client.Goal.Query().
		Where(goal.SiteID(siteID)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	names := make([]string, len(goals))
	for i, g := range goals {
		names[i] = g.DisplayName
	}
	return names, nil
}
