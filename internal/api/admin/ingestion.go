package admin

import (
	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/ingestion"
	"github.com/zenstats/zenstats/pkg/geoip"
	"github.com/zenstats/zenstats/pkg/response"
)

// GetIngestionQuality reads process-local counters independently of ClickHouse.
// The route retains the existing JWT and administrator authorization checks.
//
// @Summary 采集质量监控（当前实例）
// @Tags 管理员
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.SuccessResponse{data=ingestion.Snapshot}
// @Router /admin/ingestion-quality [get]
func (h *AdminHandler) GetIngestionQuality() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		response.Success(c, struct {
			ingestion.Snapshot
			GeoIP geoip.Status `json:"geoip"`
		}{ingestion.Default.Snapshot(), geoip.GetGeoIP().Status()})
	}
}
