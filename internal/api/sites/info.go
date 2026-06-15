package sites

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/types"
	"github.com/zenstats/zenstats/pkg/response"
)

// Info 获取站点信息
//
//	@Summary		获取站点信息
//	@Description	根据域名获取站点详细信息
//	@Tags			站点管理
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			domain	path		string												true	"站点域名"
//	@Success		200		{object}	response.SuccessResponse{data=types.SiteWithRemark}	"成功响应，返回站点信息"
//	@Failure		400		{object}	response.ErrorResponse								"请求参数错误"
//	@Failure		500		{object}	response.ErrorResponse								"服务器内部错误"
//	@Router			/sites/{domain} [get]
func (h *SitesHandler) Info() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteID := c.GetInt64("site_id")
		site, err := h.service.GetSiteByID(c, int(siteID))
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}
		SiteWithRemark := types.SiteWithRemark{
			Domain:                      site.Domain,
			ID:                          site.ID,
			IngestRateLimitScaleSeconds: site.IngestRateLimitScaleSeconds,
			IngetLimitPerMinute:         site.IngestLimitPerMinute,
			AllowedOrigins:              site.AllowedOrigins,
			Remark:                      site.Remark,
			Timezone:                    site.Timezone,
			IsVerified:                  site.IsVerified,
			VerifiedAt:                  site.VerifiedAt,
		}

		response.Success(c, SiteWithRemark)
	}
}
