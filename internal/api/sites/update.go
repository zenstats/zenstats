package sites

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/types"
	"github.com/zenstats/zenstats/pkg/response"
)

// Update 更新站点信息
//
//	@Summary		更新站点信息
//	@Description	根据域名更新站点信息
//	@Tags			站点管理
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		types.UpdateSiteRequest						true	"更新站点请求参数"
//	@Success		200		{object}	response.SuccessResponse{data=any}	"成功响应，返回更新后的站点信息"
//	@Failure		400		{object}	response.ErrorResponse						"请求参数错误"
//	@Failure		500		{object}	response.ErrorResponse						"服务器内部错误"
//	@Router			/sites/:domain [put]
func (h *SitesHandler) Update() gin.HandlerFunc {
	return func(c *gin.Context) {
		domain := c.Param("domain")
		_, err := h.service.GetSiteByDomain(c, domain)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		var req types.UpdateSiteRequest

		if err = c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		// 传递整个 req 结构体到服务层
		site, err := h.service.UpdateSite(c, domain, req)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, site)
	}
}
