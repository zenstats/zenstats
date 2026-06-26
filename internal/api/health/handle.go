// Package health 提供健康检查端点，用于 Docker 健康检查和负载均衡器探活。
package health

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	cl "github.com/zenstats/zenstats/internal/store/clickhouse"
	"github.com/zenstats/zenstats/pkg/globals"
	"github.com/zenstats/zenstats/pkg/response"
)

// Handler 健康检查处理器。
type Handler struct{}

// NewHandler 创建健康检查处理器。
func NewHandler() *Handler {
	return &Handler{}
}

// Health 返回服务健康状态，检查 PostgreSQL 和 ClickHouse 连通性（兼容旧接口，等同于 /health/ready）。
//
//	@Summary		健康检查（兼容）
//	@Description	检查服务及依赖（PostgreSQL、ClickHouse）的连通性
//	@Tags			健康检查
//	@Produce		json
//	@Success		200	{object}	response.SuccessResponse{data=HealthResponse}
//	@Failure		503	{object}	response.ErrorResponse
//	@Router			/health [get]
func (h *Handler) Health() gin.HandlerFunc {
	return h.Ready()
}

// Live 存活探针：只要进程正在运行就返回 200，不检查依赖。
// 适用于 Kubernetes liveness probe。
//
//	@Summary		存活探针
//	@Description	进程存活检查，始终返回 200
//	@Tags			健康检查
//	@Produce		json
//	@Success		200	{object}	map[string]bool
//	@Router			/health/live [get]
func (h *Handler) Live() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// Ready 就绪探针：检查 PostgreSQL 和 ClickHouse 连通性。
// 适用于 Kubernetes readiness probe。
//
//	@Summary		就绪探针
//	@Description	检查服务及依赖（PostgreSQL、ClickHouse）的连通性
//	@Tags			健康检查
//	@Produce		json
//	@Success		200	{object}	response.SuccessResponse{data=HealthResponse}
//	@Failure		503	{object}	response.ErrorResponse
//	@Router			/health/ready [get]
func (h *Handler) Ready() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		result := HealthResponse{
			Status:    "healthy",
			Timestamp: time.Now().Unix(),
		}

		db := globals.GetDB()
		if db == nil || db.Client == nil {
			result.Status = "unhealthy"
			result.Postgres = "disconnected"
		} else {
			if _, err := db.Client.User.Query().Limit(1).Count(ctx); err != nil {
				result.Status = "unhealthy"
				result.Postgres = "unreachable"
			} else {
				result.Postgres = "connected"
			}
		}

		chConn := cl.GetConnection()
		if chConn == nil {
			result.Status = "unhealthy"
			result.Clickhouse = "disconnected"
		} else {
			if err := chConn.Ping(ctx); err != nil {
				result.Status = "unhealthy"
				result.Clickhouse = "unreachable"
			} else {
				result.Clickhouse = "connected"
			}
		}

		if result.Status != "healthy" {
			response.Error(c, http.StatusServiceUnavailable, nil)
			return
		}

		response.Success(c, result)
	}
}

// HealthResponse 健康检查响应。
type HealthResponse struct {
	Status     string `json:"status"`
	Postgres   string `json:"postgres"`
	Clickhouse string `json:"clickhouse"`
	Timestamp  int64  `json:"timestamp"`
}
