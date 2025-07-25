package state

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/pkg/response"
)

// MetaStats 获取带meta筛选条件的统计数据
//
//	@Summary		获取带meta筛选条件的统计数据
//	@Description	获取指定域名的带meta筛选条件的统计数据
//	@Tags			统计分析
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			domain	path		string	true	"站点域名"
//	@Success		200		{object}	response.SuccessResponse{data=interface{}}	"成功响应，返回带meta筛选条件的统计数据"
//	@Failure		400		{object}	response.ErrorResponse				"请求参数错误"
//	@Router			/stats/{domain}/meta [get]
func (s *StateHandle) MetaStats() gin.HandlerFunc {
	return func(c *gin.Context) {
		domain := c.Param("domain")
		
		req, err := s.validate(c)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}
		
		// 解析meta筛选参数
		meta := make(map[string]string)
		for key, values := range c.Request.URL.Query() {
			if strings.HasPrefix(key, "meta[") && strings.HasSuffix(key, "]") {
				metaKey := strings.TrimSuffix(strings.TrimPrefix(key, "meta["), "]")
				meta[metaKey] = values[0]
			}
		}
		
		stats, err := s.service.GetMetaStats(c, domain, req, meta)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}
		
		response.Success(c, stats)
	}
}