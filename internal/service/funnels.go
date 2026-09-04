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

// GetFunnelService 获取 FunnelService 单例instance.
var GetFunnelService = sync.OnceValue(func() *FunnelService {
	db := globals.GetDB()
	if db == nil {
		panic("DB is not initialized")
	}
	return &FunnelService{db: db}
})

// FunnelStep 漏斗步骤数据传输对象。
type FunnelStep struct {
	ID           int64  `json:"id"`
	GoalID       int64  `json:"goal_id"`
	StepOrder    int    `json:"step_order"`
	ParentStepID *int64 `json:"parent_step_id,omitempty"`
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
	ID     int64             `json:"id"`
	SiteID int64             `json:"site_id"`
	Name   string            `json:"name"`
	Steps  []*FunnelStepInfo `json:"steps"`
}

// FunnelStepInfo 漏斗步骤详情（包含目标信息）。
type FunnelStepInfo struct {
	StepID       int64             `json:"step_id"`
	StepOrder    int               `json:"step_order"`
	ParentStepID *int64            `json:"parent_step_id,omitempty"`
	GoalID       int64             `json:"goal_id"`
	GoalName     string            `json:"goal_name"`
	GoalType     string            `json:"goal_type"`
	GoalValue    string            `json:"goal_value"`
	CustomProps  map[string]string `json:"custom_props,omitempty"`
}

// CreateFunnelRequest 创建漏斗请求参数。
type CreateFunnelRequest struct {
	Name  string            `json:"name" binding:"required"`
	Steps []FunnelStepInput `json:"steps" binding:"required,min=2,max=8"`
}

// FunnelStepInput 漏斗步骤输入参数。
// Parent 为父步骤在 steps 数组中的下标（必须小于当前下标）；
// nil 时：下标 0 视为根，其余默认接上一节点（兼容线性漏斗）。
type FunnelStepInput struct {
	GoalID int64 `json:"goal_id" binding:"required"`
	Parent *int  `json:"parent,omitempty"`
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
				ID:           fs.ID,
				GoalID:       fs.GoalID,
				StepOrder:    fs.StepOrder,
				ParentStepID: fs.ParentStepID,
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
			StepID:       fs.ID,
			StepOrder:    fs.StepOrder,
			ParentStepID: fs.ParentStepID,
			GoalID:       fs.GoalID,
			GoalName:     g.DisplayName,
			CustomProps:  g.CustomProps,
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

// resolveParentIndex 返回步骤 i 的父步骤在数组中的下标；-1 表示根节点。
// 显式 parent 必须引用更早的步骤（下标 < i），保证按数组顺序插入时父先于子。
// 未指定时：下标 0 为根，其余默认接上一节点（线性漏斗兼容）。
func resolveParentIndex(steps []FunnelStepInput, i int) (int, error) {
	if p := steps[i].Parent; p != nil {
		if *p < 0 || *p >= i {
			return -1, fmt.Errorf("step %d: parent index %d must reference an earlier step", i+1, *p)
		}
		return *p, nil
	}
	if i == 0 {
		return -1, nil
	}
	return i - 1, nil
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

	// 创建漏斗 + 步骤放在同一事务中：任一步骤写入失败时整体回滚，
	// 避免留下只有 0-1 个步骤（绕过 min=2 校验）的残缺漏斗。
	tx, err := s.db.Client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	f, err := tx.Funnel.Create().
		SetSiteID(siteID).
		SetName(req.Name).
		Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		if ent.IsConstraintError(err) {
			return nil, fmt.Errorf("funnel with name '%s' already exists", req.Name)
		}
		return nil, fmt.Errorf("failed to create funnel: %w", err)
	}

	steps := make([]*FunnelStep, len(req.Steps))
	stepIDs := make([]int64, len(req.Steps)) // 数组下标 → 新建步骤 ID，用于解析父节点
	for i, step := range req.Steps {
		parentIdx, perr := resolveParentIndex(req.Steps, i)
		if perr != nil {
			_ = tx.Rollback()
			return nil, perr
		}
		creator := tx.FunnelStep.Create().
			SetFunnelID(f.ID).
			SetGoalID(step.GoalID).
			SetStepOrder(i + 1)
		if parentIdx >= 0 {
			creator.SetParentStepID(stepIDs[parentIdx])
		}
		fs, err := creator.Save(ctx)
		if err != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("failed to create funnel step: %w", err)
		}
		stepIDs[i] = fs.ID
		steps[i] = &FunnelStep{
			ID:           fs.ID,
			GoalID:       fs.GoalID,
			StepOrder:    fs.StepOrder,
			ParentStepID: fs.ParentStepID,
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit funnel: %w", err)
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

	// 如果提供了新的步骤，先做校验（只读，不必在事务内）
	if req.Steps != nil {
		for i, step := range req.Steps {
			_, err := s.db.Client.Goal.Query().
				Where(goal.ID(step.GoalID), goal.SiteID(siteID)).
				Only(ctx)
			if err != nil {
				return nil, fmt.Errorf("goal not found for step %d", i+1)
			}
		}
		seen := make(map[int64]bool)
		for _, step := range req.Steps {
			if seen[step.GoalID] {
				return nil, fmt.Errorf("duplicate goal_id in funnel steps")
			}
			seen[step.GoalID] = true
		}
	}

	// 名称更新 + 步骤替换（删旧插新）放在同一事务，失败时不会留下空漏斗。
	tx, err := s.db.Client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	if req.Name != "" {
		if _, err := tx.Funnel.UpdateOne(existing).SetName(req.Name).Save(ctx); err != nil {
			_ = tx.Rollback()
			if ent.IsConstraintError(err) {
				return nil, fmt.Errorf("funnel with name '%s' already exists", req.Name)
			}
			return nil, fmt.Errorf("failed to update funnel: %w", err)
		}
	}
	if req.Steps != nil {
		if _, err := tx.FunnelStep.Delete().Where(funnelstep.FunnelID(funnelID)).Exec(ctx); err != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("failed to delete old funnel steps: %w", err)
		}
		stepIDs := make([]int64, len(req.Steps))
		for i, step := range req.Steps {
			parentIdx, perr := resolveParentIndex(req.Steps, i)
			if perr != nil {
				_ = tx.Rollback()
				return nil, perr
			}
			creator := tx.FunnelStep.Create().
				SetFunnelID(funnelID).
				SetGoalID(step.GoalID).
				SetStepOrder(i + 1)
			if parentIdx >= 0 {
				creator.SetParentStepID(stepIDs[parentIdx])
			}
			fs, err := creator.Save(ctx)
			if err != nil {
				_ = tx.Rollback()
				return nil, fmt.Errorf("failed to create funnel step: %w", err)
			}
			stepIDs[i] = fs.ID
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit funnel update: %w", err)
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
