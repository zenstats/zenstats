package sites

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/types"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/pkg/response"
)

// Create 创建新站点
//
//	@Summary		创建新站点
//	@Description	根据提供的参数创建一个新站点
//	@Tags			站点管理
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		types.CreateSiteRequest									true	"创建站点请求参数"
//	@Success		200		{object}	response.SuccessResponse{data=map[string]any}	"成功响应，返回创建的站点信息"
//	@Failure		400		{object}	response.ErrorResponse									"请求参数错误"
//	@Failure		500		{object}	response.ErrorResponse									"服务器内部错误"
//	@Router			/sites [post]
func (h *SitesHandler) Create() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.CreateSiteRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		// 判断 req.Timezone 是否是有效的时区
		if _, err := time.LoadLocation(req.Timezone); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		// 检查用户站点配额
		userID, _ := c.Get("user_id")
		uid := userID.(int64)
		userService := service.GetUserService()
		user, err := userService.GetUserWithConfig(c.Request.Context(), uid)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}
		if user.Edges.UserConfig != nil && user.Edges.UserConfig.Edges.Group != nil {
			maxSites := user.Edges.UserConfig.Edges.Group.MaxSites
			if maxSites != -1 {
				siteCount, err := h.service.GetUserSiteCount(c.Request.Context(), uid)
				if err != nil {
					response.Error(c, http.StatusInternalServerError, err)
					return
				}
				if siteCount >= maxSites {
					response.Error(c, http.StatusForbidden, errors.New("site limit reached for your plan"))
					return
				}
			}
		}

		// 检查该域名是否已被其他用户验证
		existingVerified, _ := h.service.GetVerifiedSiteByDomain(c.Request.Context(), req.Domain)
		if existingVerified != nil {
			response.Error(c, http.StatusConflict, fmt.Errorf("domain %s is already verified by another user, your site will not receive data", req.Domain))
			return
		}

		site, err := h.service.CreateSite(c, service.CreateSiteParams{
			Domain: req.Domain,
			// Timezone: req.Timezone,
			Timezone: "Asia/Shanghai",
			Remark:   req.Remark,
			IngestConfig: service.IngestConfig{
				RateLimitScaleSeconds: req.IngestRateLimitScaleSeconds,
				LimitPerMinute:        req.IngestLimitPerMinute,
			},
		})

		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		response.Success(c, gin.H{"site": site})
	}
}
