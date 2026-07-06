// Package panels 处理已保存的自定义查询面板（SavedPanel）的增删改查。
package panels

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/pkg/response"
)

// Handler 面板处理器。
type Handler struct{}

// NewHandler 创建面板处理器实例。
func NewHandler() *Handler { return &Handler{} }

// CreatePanelRequest 创建面板请求。
type CreatePanelRequest struct {
	Name        string `json:"name" binding:"required,max=255"`
	Description string `json:"description" binding:"max=500"`
	// Config 是自定义查询的完整配置 JSON（指标 / 维度 / 过滤器 / 排序 / limit 等）。
	Config string `json:"config" binding:"required"`
	Pinned bool   `json:"pinned"`
}

// UpdatePanelRequest 更新面板请求（字段均可选；pinned 用指针区分“未传”与 false）。
type UpdatePanelRequest struct {
	Name string `json:"name"`
	// Description 用指针区分“未传”（nil，不更新）与“传空串”（清空描述）。
	Description *string `json:"description"`
	Config      string  `json:"config"`
	Layout      string  `json:"layout"`
	Pinned      *bool   `json:"pinned"`
}

// List 获取站点的所有面板。
//
//	@Summary		获取面板列表
//	@Description	列出站点下所有已保存的自定义查询面板。可用 pinned=true 只取固定到首页的面板。
//	@Tags			面板
//	@Security		BearerAuth
//	@Produce		json
//	@Param			domain	path		string	true	"站点域名"
//	@Param			pinned	query		bool	false	"仅返回固定到首页的面板"
//	@Success		200		{object}	response.SuccessResponse{data=[]service.SavedPanel}
//	@Failure		401		{object}	response.ErrorResponse
//	@Router			/sites/{domain}/panels [get]
func (h *Handler) List() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteID := c.GetInt64("site_id")
		var (
			panels []*service.SavedPanel
			err    error
		)
		if c.Query("pinned") == "true" {
			panels, err = service.GetSavedPanelService().ListPinned(c, siteID)
		} else {
			panels, err = service.GetSavedPanelService().List(c, siteID)
		}
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}
		response.Success(c, panels)
	}
}

// Create 创建新面板。
//
//	@Summary		创建面板
//	@Description	保存一次自定义查询的完整配置为命名面板，之后可复用或固定到首页。
//	@Tags			面板
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			domain	path		string				true	"站点域名"
//	@Param			body	body		CreatePanelRequest	true	"面板参数"
//	@Success		200		{object}	response.SuccessResponse{data=service.SavedPanel}
//	@Failure		400		{object}	response.ErrorResponse
//	@Router			/sites/{domain}/panels [post]
func (h *Handler) Create() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteID := c.GetInt64("site_id")
		userID := c.GetInt64("user_id")

		var req CreatePanelRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		panel, err := service.GetSavedPanelService().Create(c, siteID, userID, req.Name, req.Description, req.Config, req.Pinned)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, panel)
	}
}

// Update 更新面板（名称 / 描述 / 配置 / 是否固定）。
//
//	@Summary		更新面板
//	@Tags			面板
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			domain	path		string				true	"站点域名"
//	@Param			panelId	path		string				true	"面板 ID"
//	@Param			body	body		UpdatePanelRequest	true	"更新参数"
//	@Success		200		{object}	response.SuccessResponse{data=service.SavedPanel}
//	@Router			/sites/{domain}/panels/{panelId} [patch]
func (h *Handler) Update() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteID := c.GetInt64("site_id")
		panelID, err := parseID(c, "panelId")
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		var req UpdatePanelRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		panel, err := service.GetSavedPanelService().Update(c, siteID, panelID, req.Name, req.Description, req.Config, req.Layout, req.Pinned)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, panel)
	}
}

// Delete 删除面板。
//
//	@Summary		删除面板
//	@Tags			面板
//	@Security		BearerAuth
//	@Param			domain	path		string	true	"站点域名"
//	@Param			panelId	path		string	true	"面板 ID"
//	@Success		200		{object}	response.SuccessResponse{data=map[string]bool}
//	@Router			/sites/{domain}/panels/{panelId} [delete]
func (h *Handler) Delete() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteID := c.GetInt64("site_id")
		panelID, err := parseID(c, "panelId")
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		if err := service.GetSavedPanelService().Delete(c, siteID, panelID); err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, gin.H{"deleted": true})
	}
}

func parseID(c *gin.Context, param string) (int64, error) {
	s := c.Param(param)
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %s", param, s)
	}
	return id, nil
}
