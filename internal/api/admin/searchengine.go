package admin

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/pkg/response"
)

// SearchEngineItem 内置来源 CRUD 的请求/响应结构体
type SearchEngineItem struct {
	ID     int64  `json:"id,omitempty"`
	Domain string `json:"domain" binding:"required"`
	Name   string `json:"name" binding:"required"`
}

// SearchEngineHandler 管理员管理内置来源
type SearchEngineHandler struct {
	service *service.SearchEngineService
}

func NewSearchEngineHandler() *SearchEngineHandler {
	return &SearchEngineHandler{
		service: service.GetSearchEngineService(),
	}
}

// ListSearchEngines 返回所有内置来源
func (h *SearchEngineHandler) ListSearchEngines() gin.HandlerFunc {
	return func(c *gin.Context) {
		engines, err := h.service.ListAll(c.Request.Context())
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}
		items := make([]SearchEngineItem, 0, len(engines))
		for _, e := range engines {
			items = append(items, SearchEngineItem{
				ID:     e.ID,
				Domain: e.Domain,
				Name:   e.Name,
			})
		}
		response.Success(c, items)
	}
}

// CreateSearchEngine 新增内置来源
func (h *SearchEngineHandler) CreateSearchEngine() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req SearchEngineItem
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}
		engine, err := h.service.Create(c.Request.Context(), req.Domain, req.Name)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}
		response.Success(c, SearchEngineItem{ID: engine.ID, Domain: engine.Domain, Name: engine.Name})
	}
}

// UpdateSearchEngine 修改内置来源
func (h *SearchEngineHandler) UpdateSearchEngine() gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}
		var req SearchEngineItem
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}
		engine, err := h.service.Update(c.Request.Context(), id, req.Domain, req.Name)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}
		response.Success(c, SearchEngineItem{ID: engine.ID, Domain: engine.Domain, Name: engine.Name})
	}
}

// DeleteSearchEngine 删除内置来源
func (h *SearchEngineHandler) DeleteSearchEngine() gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}
		if err := h.service.Delete(c.Request.Context(), id); err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}
		response.Success(c, gin.H{"message": "deleted"})
	}
}
