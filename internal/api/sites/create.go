package sites

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/types"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/pkg/response"
)

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
