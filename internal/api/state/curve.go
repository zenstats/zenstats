package state

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/pkg/response"
)

// GetCurve 获取统计曲线数据
//
//	@Summary		获取统计曲线数据
//	@Description	获取指定域名的统计曲线数据
//	@Tags			统计分析
//	@Security		BearerAuth
//	@Accept			json
//
//	@Produce		json
//	@Param			request	body		types.StatsRequest						true	"参数"
//	@Param			domain	path		string										true	"站点域名"
//	@Success		200		{object}	response.SuccessResponse{data=any}	"成功响应，返回统计曲线数据"
//	@Failure		400		{object}	response.ErrorResponse						"请求参数错误"
//	@Router			/stats/{domain}/curve [get]
func (s *StateHandle) GetCurve() gin.HandlerFunc {
	return func(c *gin.Context) {
		domain := c.Param("domain")

		req, err := s.validate(c)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		stats, err := s.service.GetCurve(c, domain, req)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		response.Success(c, stats)
	}
}
