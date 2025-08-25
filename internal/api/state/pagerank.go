package state

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/pkg/response"
)

// GetPageRank 获取页面排名统计
//
//	@Summary		获取页面排名统计
//	@Description	获取指定域名的页面排名统计数据
//	@Tags			统计分析
//	@Security		BearerAuth
//	@Accept			json
//
//	@Produce		json
//	@Param			domain	path		string										true	"站点域名"
//	@Success		200		{object}	response.SuccessResponse{data=any}	"成功响应，返回页面排名统计数据"
//	@Failure		400		{object}	response.ErrorResponse						"请求参数错误"
//	@Router			/stats/{domain}/page_rank [get]
func (s *StateHandle) GetPageRank() gin.HandlerFunc {
	return func(c *gin.Context) {
		domain := c.Param("domain")

		req, err := s.validate(c)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		stats, err := s.service.GetPageRank(c, domain, req)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		response.Success(c, stats)
	}
}
