package sites

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/pkg/response"
)

// Delete 删除站点
//
//	@Summary		删除站点
//	@Description	根据域名删除指定站点
//	@Tags			站点管理
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			domain	path		string								true	"站点域名"
//	@Success		200		{object}	response.SuccessResponse{data=nil}	"成功响应"
//	@Failure		400		{object}	response.ErrorResponse				"请求参数错误"
//	@Failure		401		{object}	response.ErrorResponse				"未认证或认证失败"
//	@Failure		500		{object}	response.ErrorResponse				"服务器内部错误"
//	@Router			/sites/{domain} [delete]
func (h *SitesHandler) Delete() gin.HandlerFunc {
	return func(c *gin.Context) {
		domain := c.Param("domain")
		site, err := h.service.GetSiteByDomain(c, domain)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		if err := h.service.DeleteSite(c, int(site.ID)); err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, nil)
	}
}
