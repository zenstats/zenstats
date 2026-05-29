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
//	@Accept			json
//	@Produce		json
//	@Param			domain	path		string	true	"站点域名"
//	@Success		200		{object}	response.SuccessResponse{data=any}	"成功响应"
//	@Failure		400		{object}	response.ErrorResponse	"请求参数错误"
//	@Router			/stats/{domain}/current-visitors [get]
func (s *StatsHandle) GetCurrentVisitors() gin.HandlerFunc {
	return func(c *gin.Context) {
		domain := c.Param("domain")

		result, err := s.statsService.GetCurrentVisitors(c, domain)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, result)
	}
}
