package admin

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/types"
	"github.com/zenstats/zenstats/pkg/response"
)

// ListSites 获取站点列表
//
//	@Summary		获取站点列表
//	@Description	获取所有站点的列表，支持分页。
//	@Tags			管理员
//	@Accept			json
//	@Produce		json
//	@Param			page		query		int	false	"页码"		default(1)
//	@Param			page_size	query		int	false	"每页数量"	default(20)
//	@Success		200			{object}	response.SuccessResponse{data=types.AdminSiteListResponse}	"站点列表"
//	@Failure		401			{object}	response.ErrorResponse	"未授权"
//	@Failure		500			{object}	response.ErrorResponse	"服务器内部错误"
//	@Router			/admin/sites [get]
func (h *AdminHandler) ListSites() gin.HandlerFunc {
	return func(c *gin.Context) {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

		if page < 1 {
			page = 1
		}
		if pageSize < 1 || pageSize > 100 {
			pageSize = 20
		}

		offset := (page - 1) * pageSize

		sites, err := h.siteService.GetAllSitesWithOwner(c.Request.Context(), offset, pageSize)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		totalCount, err := h.siteService.GetAllSitesCount(c.Request.Context())
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		siteList := make([]*types.AdminSite, 0, len(sites))
		for _, s := range sites {
			siteList = append(siteList, &types.AdminSite{
				ID:         s.ID,
				Domain:     s.Domain,
				Remark:     s.Remark,
				Timezone:   s.Timezone,
				OwnerName:  s.OwnerName,
				IsVerified: s.IsVerified,
				VerifiedAt: s.VerifiedAt,
				CreatedAt:  s.CreatedAt,
			})
		}

		response.Success(c, &types.AdminSiteListResponse{
			Sites:      siteList,
			TotalCount: totalCount,
			Page:       page,
			PageSize:   pageSize,
		})
	}
}

// DeleteSite 删除站点
//
//	@Summary		删除站点
//	@Description	管理员删除指定站点。
//	@Tags			管理员
//	@Accept			json
//	@Produce		json
//	@Param			siteId	path		int	true	"站点ID"
//	@Success		200		{object}	response.SuccessResponse	"删除成功"
//	@Failure		400		{object}	response.ErrorResponse	"请求参数错误"
//	@Failure		401		{object}	response.ErrorResponse	"未授权"
//	@Failure		404		{object}	response.ErrorResponse	"站点不存在"
//	@Failure		500		{object}	response.ErrorResponse	"服务器内部错误"
//	@Router			/admin/sites/{siteId} [delete]
func (h *AdminHandler) DeleteSite() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteIdStr := c.Param("siteId")
		siteId, err := strconv.ParseInt(siteIdStr, 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		if err := h.siteService.DeleteSite(c, int(siteId)); err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, nil)
	}
}

// VerifySite 管理员手动验证站点
//
//	@Summary		手动验证站点
//	@Description	管理员手动将站点标记为已验证（跳过域名所有权验证）。
//	@Tags			管理员
//	@Accept			json
//	@Produce		json
//	@Param			siteId	path		int	true	"站点ID"
//	@Success		200		{object}	response.SuccessResponse	"验证成功"
//	@Failure		400		{object}	response.ErrorResponse	"请求参数错误"
//	@Failure		401		{object}	response.ErrorResponse	"未授权"
//	@Failure		404		{object}	response.ErrorResponse	"站点不存在"
//	@Failure		500		{object}	response.ErrorResponse	"服务器内部错误"
//	@Router			/admin/sites/{siteId}/verify [put]
func (h *AdminHandler) VerifySite() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteIdStr := c.Param("siteId")
		siteId, err := strconv.ParseInt(siteIdStr, 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		if err := h.siteService.AdminVerifySite(c.Request.Context(), siteId); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		response.Success(c, nil)
	}
}
