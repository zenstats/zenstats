package stats

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/internal/service/stats/types"
	"github.com/zenstats/zenstats/pkg/response"
)

// SaveQuerySegmentRequest 将当前查询条件保存为 Segment 的请求体。
// filters 使用与查询接口 (QueryV2) 完全一致的过滤器数组结构，前端可直接透传当前查询用的 filters。
type SaveQuerySegmentRequest struct {
	Name        string          `json:"name" binding:"required,max=255"`
	Description string          `json:"description" binding:"max=500"`
	Filters     json.RawMessage `json:"filters" binding:"required"`
}

// SaveQuerySegment 将当前查询使用的过滤条件一键保存为命名 Segment，便于后续复用。
//
//	@Summary		保存当前查询为 Segment
//	@Description	把当前自定义查询使用的 filters 一键保存为命名的过滤器组合（Segment），之后可在 Segment 列表中复用。filters 结构与 /api/v2/query 的 filters 完全一致。
//	@Tags			统计分析
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Param			domain	path		string						true	"站点域名"
//	@Param			body	body		SaveQuerySegmentRequest		true	"保存参数"
//	@Success		200		{object}	response.SuccessResponse{data=service.Segment}
//	@Failure		400		{object}	response.ErrorResponse
//	@Failure		401		{object}	response.ErrorResponse
//	@Failure		500		{object}	response.ErrorResponse
//	@Router			/api/v2/query/{domain}/segments [post]
func (s *StatsHandle) SaveQuerySegment() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteID := c.GetInt64("site_id")
		userID := c.GetInt64("user_id")

		var req SaveQuerySegmentRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		// 校验 filters 结构合法：复用查询接口同一套解析逻辑，避免保存出无法查询的条件。
		filtersStr := string(req.Filters)
		if _, err := types.ParseRawFiltersJSON(filtersStr); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		seg, err := service.GetSegmentService().Create(c, siteID, userID, req.Name, req.Description, filtersStr)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, seg)
	}
}
