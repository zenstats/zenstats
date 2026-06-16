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

// Health 返回服务健康状态，检查 PostgreSQL 和 ClickHouse 连通性。
//
//	@Summary		健康检查
//	@Description	检查服务及依赖（PostgreSQL、ClickHouse）的连通性
//	@Tags			健康检查
//	@Produce		json
//	@Success		200	{object}	response.SuccessResponse{data=HealthResponse}
//	@Failure		503	{object}	response.ErrorResponse
//	@Router			/health [get]
func (h *Handler) Health() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		result := HealthResponse{
			Status:    "healthy",
			Timestamp: time.Now().Unix(),
		}

		// 检查 PostgreSQL：ent ORM 通过 User 表做轻量探测
		db := globals.GetDB()
		if db == nil || db.Client == nil {
			result.Status = "unhealthy"
			result.Postgres = "disconnected"
		} else {
			// 使用轻量 Count 查询验证 DB 连通性，不依赖具体数据
			if _, err := db.Client.User.Query().Limit(1).Count(ctx); err != nil {
				result.Status = "unhealthy"
				result.Postgres = "unreachable"
			} else {
				result.Postgres = "connected"
			}
		}

		// 检查 ClickHouse
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
