package sites

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/pkg/response"
)

// List 获取站点列表
//
//	@Summary		获取站点列表
//	@Description	根据域名查询用户站点列表
//	@Tags			站点管理
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//
//
//	@Param			domain	query		string											false	"站点域名（模糊查询）"
//	@Success		200		{object}	response.SuccessResponse{data=[]any}	"成功响应，返回站点列表"
//	@Failure		400		{object}	response.ErrorResponse							"请求参数错误"
//	@Router			/sites [get]
func (h *SitesHandler) List() gin.HandlerFunc {
	return func(c *gin.Context) {
		domain := c.Query("domain")
		list, err := h.service.GetUserSiteByDomain(c, domain)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}
		response.Success(c, list)
	}
}
