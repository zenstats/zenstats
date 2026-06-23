package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/funnel"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/funnelstep"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/goal"
	"github.com/zenstats/zenstats/pkg/globals"
)

// FunnelService 漏斗管理服务，提供漏斗的增删改查功能。
type FunnelService struct {
	db *postgresql.Client
}

// GetFunnelService 获取 FunnelService 单例实例。
var GetFunnelService = sync.OnceValue(func() *FunnelService {
	db := globals.GetDB()
	if db == nil {
		panic("DB is not initialized")
	}
	return &FunnelService{db: db}
})

// FunnelStep 漏斗步骤数据传输对象。
type FunnelStep struct {
	ID       int64 `json:"id"`
	GoalID   int64 `json:"goal_id"`
	StepOrder int   `json:"step_order"`
}

// Funnel 漏斗数据传输对象。
type Funnel struct {
	ID        int64         `json:"id"`
	SiteID    int64         `json:"site_id"`
	Name      string        `json:"name"`
	Steps     []*FunnelStep `json:"steps"`
	CreatedAt string        `json:"created_at,omitempty"`
}

// FunnelDetail 漏斗详情（包含步骤的目标信息）。
type FunnelDetail struct {
	ID    int64             `json:"id"`
	SiteID int64            `json:"site_id"`
	Name  string            `json:"name"`
	Steps []*FunnelStepInfo `json:"steps"`
}

// FunnelStepInfo 漏斗步骤详情（包含目标信息）。
type FunnelStepInfo struct {
	StepOrder   int               `json:"step_order"`
	GoalID      int64             `json:"goal_id"`
	GoalName    string            `json:"goal_name"`
	GoalType    string            `json:"goal_type"`
	GoalValue   string            `json:"goal_value"`
	CustomProps map[string]string `json:"custom_props,omitempty"`
}

// CreateFunnelRequest 创建漏斗请求参数。
type CreateFunnelRequest struct {
	Name  string            `json:"name" binding:"required"`
	Steps []FunnelStepInput `json:"steps" binding:"required,min=2,max=8"`
}

// FunnelStepInput 漏斗步骤输入参数。
type FunnelStepInput struct {
	GoalID int64 `json:"goal_id" binding:"required"`
}

// UpdateFunnelRequest 更新漏斗请求参数。
type UpdateFunnelRequest struct {
	Name  string            `json:"name" binding:"omitempty"`
	Steps []FunnelStepInput `json:"steps" binding:"omitempty,min=2,max=8"`
}

// ListFunnels 获取站点的所有漏斗。
func (s *FunnelService) ListFunnels(ctx context.Context, siteID int64) ([]*Funnel, error) {
	funnels, err := s.db.Client.Funnel.Query().
		Where(funnel.SiteID(siteID)).
		WithFunnelSteps(func(q *ent.FunnelStepQuery) {
			q.Order(ent.Asc(funnelstep.FieldStepOrder))
		}).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list funnels: %w", err)
	}

	result := make([]*Funnel, len(funnels))
	for i, f := range funnels {
		steps := make([]*FunnelStep, len(f.Edges.FunnelSteps))
		for j, fs := range f.Edges.FunnelSteps {
			steps[j] = &FunnelStep{
				ID:        fs.ID,
				GoalID:    fs.GoalID,
				StepOrder: fs.StepOrder,
			}
		}
		result[i] = &Funnel{
			ID:     f.ID,
			SiteID: f.SiteID,
			Name:   f.Name,
			Steps:  steps,
		}
	}
	return result, nil
}

// GetFunnel 获取单个漏斗详情。
func (s *FunnelService) GetFunnel(ctx context.Context, siteID int64, funnelID int64) (*FunnelDetail, error) {
	f, err := s.db.Client.Funnel.Query().
		Where(funnel.ID(funnelID), funnel.SiteID(siteID)).
		WithFunnelSteps(func(q *ent.FunnelStepQuery) {
			q.Order(ent.Asc(funnelstep.FieldStepOrder))
		}).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("funnel not found")
	}

	steps := make([]*FunnelStepInfo, len(f.Edges.FunnelSteps))
	for i, fs := range f.Edges.FunnelSteps {
		g, err := s.db.Client.Goal.Get(ctx, fs.GoalID)
		if err != nil {
			continue
		}

		stepInfo := &FunnelStepInfo{
			StepOrder:   fs.StepOrder,
			GoalID:      fs.GoalID,
			GoalName:    g.DisplayName,
			CustomProps: g.CustomProps,
		}

		if g.EventName != "" {
			stepInfo.GoalType = "event"
			stepInfo.GoalValue = g.EventName
		} else {
			stepInfo.GoalType = "page"
			stepInfo.GoalValue = g.PagePath
		}

		steps[i] = stepInfo
	}

	return &FunnelDetail{
		ID:     f.ID,
		SiteID: f.SiteID,
		Name:   f.Name,
		Steps:  steps,
	}, nil
}

// CreateFunnel 创建新漏斗。
func (s *FunnelService) CreateFunnel(ctx context.Context, siteID int64, req *CreateFunnelRequest) (*Funnel, error) {
	// 验证所有 goal_id 存在且属于该站点
	for i, step := range req.Steps {
		_, err := s.db.Client.Goal.Query().
			Where(goal.ID(step.GoalID), goal.SiteID(siteID)).
			Only(ctx)
		if err != nil {
			return nil, fmt.Errorf("goal not found for step %d", i+1)
		}
	}

	// 检查步骤是否有重复的 goal_id
	seen := make(map[int64]bool)
	for _, step := range req.Steps {
		if seen[step.GoalID] {
			return nil, fmt.Errorf("duplicate goal_id in funnel steps")
		}
		seen[step.GoalID] = true
	}

	// 创建漏斗
	f, err := s.db.Client.Funnel.Create().
		SetSiteID(siteID).
		SetName(req.Name).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, fmt.Errorf("funnel with name '%s' already exists", req.Name)
		}
		return nil, fmt.Errorf("failed to create funnel: %w", err)
	}

	// 创建漏斗步骤
	steps := make([]*FunnelStep, len(req.Steps))
	for i, step := range req.Steps {
		fs, err := s.db.Client.FunnelStep.Create().
			SetFunnelID(f.ID).
			SetGoalID(step.GoalID).
			SetStepOrder(i + 1).
			Save(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create funnel step: %w", err)
		}
		steps[i] = &FunnelStep{
			ID:        fs.ID,
			GoalID:    fs.GoalID,
			StepOrder: fs.StepOrder,
		}
	}

	return &Funnel{
		ID:     f.ID,
		SiteID: f.SiteID,
		Name:   f.Name,
		Steps:  steps,
	}, nil
}

// UpdateFunnel 更新漏斗。
func (s *FunnelService) UpdateFunnel(ctx context.Context, siteID int64, funnelID int64, req *UpdateFunnelRequest) (*FunnelDetail, error) {
	// 获取现有漏斗
	existing, err := s.db.Client.Funnel.Query().
		Where(funnel.ID(funnelID), funnel.SiteID(siteID)).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("funnel not found")
	}

	// 更新名称
	if req.Name != "" {
		existing, err = existing.Update().
			SetName(req.Name).
			Save(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to update funnel: %w", err)
		}
	}

	// 如果提供了新的步骤，替换所有步骤
	if req.Steps != nil {
		// 验证所有 goal_id
		for i, step := range req.Steps {
			_, err := s.db.Client.Goal.Query().
				Where(goal.ID(step.GoalID), goal.SiteID(siteID)).
				Only(ctx)
			if err != nil {
				return nil, fmt.Errorf("goal not found for step %d", i+1)
			}
		}

		// 检查重复
		seen := make(map[int64]bool)
		for _, step := range req.Steps {
			if seen[step.GoalID] {
				return nil, fmt.Errorf("duplicate goal_id in funnel steps")
			}
			seen[step.GoalID] = true
		}

		// 删除旧步骤
		_, err = s.db.Client.FunnelStep.Delete().
			Where(funnelstep.FunnelID(funnelID)).
			Exec(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to delete old funnel steps: %w", err)
		}

		// 创建新步骤
		for i, step := range req.Steps {
			_, err := s.db.Client.FunnelStep.Create().
				SetFunnelID(funnelID).
				SetGoalID(step.GoalID).
				SetStepOrder(i + 1).
				Save(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to create funnel step: %w", err)
			}
		}
	}

	// 返回更新后的漏斗
	return s.GetFunnel(ctx, siteID, funnelID)
}

// DeleteFunnel 删除漏斗。
func (s *FunnelService) DeleteFunnel(ctx context.Context, siteID int64, funnelID int64) error {
	// 先删除关联的漏斗步骤（外键约束）
	_, err := s.db.Client.FunnelStep.Delete().
		Where(funnelstep.FunnelID(funnelID)).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete funnel steps: %w", err)
	}
	err = s.db.Client.Funnel.DeleteOneID(funnelID).
		Where(funnel.SiteID(siteID)).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete funnel: %w", err)
	}
	return nil
}
