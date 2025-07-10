package state

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/pkg/response"
)

// GetTopStats 获取顶级统计数据
//
//	@Summary		获取顶级统计数据
//	@Description	获取指定域名的顶级统计数据
//	@Tags			统计分析
//	@Security		BearerAuth
//	@Accept			json
//
//	@Produce		json
//	@Param			domain	path		string										true	"站点域名"
//	@Success		200		{object}	response.SuccessResponse{data=interface{}}	"成功响应，返回顶级统计数据"
//	@Failure		400		{object}	response.ErrorResponse						"请求参数错误"
//	@Router			/stats/{domain}/top_stats [get]
func (s *StateHandle) GetTopStats() gin.HandlerFunc {
	return func(c *gin.Context) {
		domain := c.Param("domain")

		req, err := s.validate(c)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		stats, err := s.service.GetTopStats(c, domain, req)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		response.Success(c, stats)
	}
}
