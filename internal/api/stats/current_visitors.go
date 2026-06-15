package stats

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/pkg/response"
)

// GetCurrentVisitors 获取实时在线访客数（对标 Plausible /api/v1/stats/realtime/visitors）
//
//	@Summary		获取实时在线访客数
//	@Description	获取当前站点最近5分钟内的实时在线访客和会话数
//	@Tags			统计分析
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Accept			json
//	@Produce		json
//	@Param			domain	path		string	true	"站点域名，例如 example.com"
//	@Success		200		{object}	response.SuccessResponse{data=any}	"成功响应，data 为实时访客数和会话数"
//	@Failure		401		{object}	response.ErrorResponse	"未认证或认证失败"
//	@Failure		500		{object}	response.ErrorResponse	"服务器内部错误"
//	@Router			/stats/{domain}/current-visitors [get]
func (s *StatsHandle) GetCurrentVisitors() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteID := c.GetInt64("site_id")

		result, err := s.statsService.GetCurrentVisitors(c, siteID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, result)
	}
}
