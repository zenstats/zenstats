package state

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/pkg/response"
)

// GetDeviceRank 获取设备排名统计
//
//	@Summary		获取设备排名统计
//	@Description	获取指定域名的设备排名统计数据
//	@Tags			统计分析
//
//	@Security		BearerAuth
//	@Accept			json
//
//	@Produce		json
//	@Param			domain	path		string										true	"站点域名"
//	@Success		200		{object}	response.SuccessResponse{data=interface{}}	"成功响应，返回设备排名统计数据"
//	@Failure		400		{object}	response.ErrorResponse						"请求参数错误"
//	@Router			/stats/{domain}/device_rank [get]
func (s *StateHandle) GetDeviceRank() gin.HandlerFunc {
	return func(c *gin.Context) {
		domain := c.Param("domain")

		req, err := s.validate(c)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		stats, err := s.service.GetDeviceRank(c, domain, req)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		response.Success(c, stats)
	}
}
