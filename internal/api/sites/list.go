package sites

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/pkg/response"
)

// List 获取站点列表
//
//	@Summary		获取站点列表
//	@Description	获取当前用户可访问的站点列表，可按域名进行模糊查询。
//	@Tags			站点管理
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//
//
//	@Param			domain	query		string											false	"站点域名（模糊查询）"
//	@Success		200		{object}	response.SuccessResponse{data=[]service.SiteWithRemark}	"成功响应，返回站点列表"
//	@Failure		400		{object}	response.ErrorResponse							"请求参数错误"
//	@Failure		401		{object}	response.ErrorResponse							"未认证或认证失败"
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
