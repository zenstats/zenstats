// Package segments 处理过滤器组合（Segment）的增删改查。
package segments

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/pkg/response"
)

// Handler Segment 处理器。
type Handler struct{}

// NewHandler 创建 Segment 处理器实例。
func NewHandler() *Handler { return &Handler{} }

// CreateSegmentRequest 创建 Segment 请求。
type CreateSegmentRequest struct {
	Name        string `json:"name" binding:"required,max=255"`
	Description string `json:"description" binding:"max=500"`
	Filters     string `json:"filters" binding:"required"`
}

// UpdateSegmentRequest 更新 Segment 请求（字段均可选）。
type UpdateSegmentRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Filters     string `json:"filters"`
}

// List 获取站点的所有 Segment。
//
//	@Summary		获取 Segment 列表
//	@Description	列出站点下所有已保存的过滤器组合（Segment）。
//	@Tags			Segment
//	@Security		BearerAuth
//	@Produce		json
//	@Param			domain	path		string	true	"站点域名"
//	@Success		200		{object}	response.SuccessResponse{data=[]service.Segment}
//	@Failure		401		{object}	response.ErrorResponse
//	@Router			/sites/{domain}/segments [get]
func (h *Handler) List() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteID := c.GetInt64("site_id")
		segs, err := service.GetSegmentService().List(c, siteID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}
		response.Success(c, segs)
	}
}

// Create 创建新 Segment。
//
//	@Summary		创建 Segment
//	@Description	为站点保存一组命名的过滤器组合，之后可通过 segment_id 在查询中复用。
//	@Tags			Segment
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			domain	path		string					true	"站点域名"
//	@Param			body	body		CreateSegmentRequest	true	"Segment 参数"
//	@Success		201		{object}	response.SuccessResponse{data=service.Segment}
//	@Failure		400		{object}	response.ErrorResponse
//	@Router			/sites/{domain}/segments [post]
func (h *Handler) Create() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteID := c.GetInt64("site_id")
		userID := c.GetInt64("user_id")

		var req CreateSegmentRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		seg, err := service.GetSegmentService().Create(c, siteID, userID, req.Name, req.Description, req.Filters)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, seg)
	}
}

// Update 更新 Segment。
//
//	@Summary		更新 Segment
//	@Description	更新已有 Segment 的名称或过滤器定义。
//	@Tags			Segment
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			domain		path		string					true	"站点域名"
//	@Param			segmentId	path		string					true	"Segment ID"
//	@Param			body		body		UpdateSegmentRequest	true	"更新参数"
//	@Success		200			{object}	response.SuccessResponse{data=service.Segment}
//	@Router			/sites/{domain}/segments/{segmentId} [patch]
func (h *Handler) Update() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteID := c.GetInt64("site_id")
		segID, err := parseID(c, "segmentId")
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		var req UpdateSegmentRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		seg, err := service.GetSegmentService().Update(c, siteID, segID, req.Name, req.Description, req.Filters)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, seg)
	}
}

// Delete 删除 Segment。
//
//	@Summary		删除 Segment
//	@Tags			Segment
//	@Security		BearerAuth
//	@Param			domain		path		string	true	"站点域名"
//	@Param			segmentId	path		string	true	"Segment ID"
//	@Success		200			{object}	response.SuccessResponse{data=map[string]bool}
//	@Router			/sites/{domain}/segments/{segmentId} [delete]
func (h *Handler) Delete() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteID := c.GetInt64("site_id")
		segID, err := parseID(c, "segmentId")
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		if err := service.GetSegmentService().Delete(c, siteID, segID); err != nil {
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
