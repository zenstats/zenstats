package state

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/pkg/response"
)

// GetSourceRank 获取来源排名统计
//
//	@Summary		获取来源排名统计
//	@Description	获取指定域名的来源排名统计数据
//	@Tags			统计分析
//	@Security		BearerAuth
//	@Accept			json
//
//	@Produce		json
//	@Param			domain	path		string										true	"站点域名"
//	@Success		200		{object}	response.SuccessResponse{data=any}	"成功响应，返回来源排名统计数据"
//	@Failure		400		{object}	response.ErrorResponse						"请求参数错误"
//	@Router			/stats/{domain}/source_rank [get]
func (s *StateHandle) GetSourceRank() gin.HandlerFunc {
	return func(c *gin.Context) {
		domain := c.Param("domain")

		req, err := s.validate(c)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}
		service := service.GetStatsService()

		stats, err := service.GetSourceRank(c, domain, req)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		response.Success(c, stats)
	}
}

func (s *StateHandle) GetAggregate() gin.HandlerFunc {
	return func(c *gin.Context) {
		domain := c.Param("domain")

		req, err := s.validate(c)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		service := service.GetStatsService()
		stats, err := service.GetAggregate(c, domain, req)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		response.Success(c, stats)
	}
}

func (s *StateHandle) GetTimeSeries() gin.HandlerFunc {
	return func(c *gin.Context) {
		domain := c.Param("domain")

		req, err := s.validate(c)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		service := service.GetStatsService()
		stats, err := service.GetTimeSeries(c, domain, req)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		response.Success(c, stats)
	}
}
