package user

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/types"
	"github.com/zenstats/zenstats/pkg/response"
)

// @Summary		获取自定义搜索引擎列表
// @Description	获取当前用户的自定义搜索引擎列表。
// @Tags			用户
// @Accept			json
// @Produce		json
// @Success		200	{object}	response.SuccessResponse{data=types.CustomSearchEngineListResponse}	"搜索引擎列表"
// @Failure		401	{object}	response.ErrorResponse	"未授权"
// @Failure		500	{object}	response.ErrorResponse	"服务器内部错误"
// @Router			/user/search-engines [get]
func (h *UserHandler) ListSearchEngines() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetInt64("user_id")

		// 检查用户是否有自定义搜索引擎权限
		hasPermission, err := h.customSearchEngineService.HasSearchEnginePermission(c.Request.Context(), userID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		engines, err := h.customSearchEngineService.GetUserSearchEngines(c.Request.Context(), userID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		// 转换为API响应格式
		engineList := make([]*types.CustomSearchEngine, 0, len(engines))
		for _, e := range engines {
			engineList = append(engineList, &types.CustomSearchEngine{
				ID:        e.ID,
				Domain:    e.Domain,
				Name:      e.Name,
				CreatedAt: e.CreatedAt,
				UpdatedAt: e.UpdatedAt,
			})
		}

		response.Success(c, &types.CustomSearchEngineListResponse{
			Engines:       engineList,
			HasPermission: hasPermission,
		})
	}
}

// @Summary		创建自定义搜索引擎
// @Description	为当前用户创建自定义搜索引擎。
// @Tags			用户
// @Accept			json
// @Produce		json
// @Param			request	body		types.CreateSearchEngineRequest	true	"搜索引擎配置"
// @Success		200		{object}	response.SuccessResponse{data=types.CustomSearchEngine}	"创建成功"
// @Failure		400		{object}	response.ErrorResponse		"请求参数错误"
// @Failure		401		{object}	response.ErrorResponse		"未授权"
// @Failure		403		{object}	response.ErrorResponse		"无权限"
// @Failure		500		{object}	response.ErrorResponse		"服务器内部错误"
// @Router			/user/search-engines [post]
func (h *UserHandler) CreateSearchEngine() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetInt64("user_id")

		// 检查用户是否有自定义搜索引擎权限
		hasPermission, err := h.customSearchEngineService.HasSearchEnginePermission(c.Request.Context(), userID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}
		if !hasPermission {
			response.Error(c, http.StatusForbidden, ErrNoPermission)
			return
		}

		var req types.CreateSearchEngineRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		engine, err := h.customSearchEngineService.CreateSearchEngine(c.Request.Context(), userID, req.Domain, req.Name)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, &types.CustomSearchEngine{
			ID:        engine.ID,
			Domain:    engine.Domain,
			Name:      engine.Name,
			CreatedAt: engine.CreatedAt,
			UpdatedAt: engine.UpdatedAt,
		})
	}
}

// @Summary		更新自定义搜索引擎
// @Description	更新指定的自定义搜索引擎。
// @Tags			用户
// @Accept			json
// @Produce		json
// @Param			id		path		int								true	"搜索引擎ID"
// @Param			request	body		types.UpdateSearchEngineRequest	true	"搜索引擎配置"
// @Success		200		{object}	response.SuccessResponse		"更新成功"
// @Failure		400		{object}	response.ErrorResponse		"请求参数错误"
// @Failure		401		{object}	response.ErrorResponse		"未授权"
// @Failure		403		{object}	response.ErrorResponse		"无权限"
// @Failure		404		{object}	response.ErrorResponse		"搜索引擎不存在"
// @Failure		500		{object}	response.ErrorResponse		"服务器内部错误"
// @Router			/user/search-engines/{id} [put]
func (h *UserHandler) UpdateSearchEngine() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetInt64("user_id")
		engineIDStr := c.Param("id")
		engineID, err := strconv.ParseInt(engineIDStr, 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		// 检查用户是否有自定义搜索引擎权限
		hasPermission, err := h.customSearchEngineService.HasSearchEnginePermission(c.Request.Context(), userID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}
		if !hasPermission {
			response.Error(c, http.StatusForbidden, ErrNoPermission)
			return
		}

		// 检查搜索引擎是否属于当前用户
		engine, err := h.customSearchEngineService.GetSearchEngineByID(c.Request.Context(), engineID)
		if err != nil {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		if engine.UserID != userID {
			response.Error(c, http.StatusForbidden, ErrNotOwner)
			return
		}

		var req types.UpdateSearchEngineRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		_, err = h.customSearchEngineService.UpdateSearchEngine(c.Request.Context(), engineID, req.Domain, req.Name)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, nil)
	}
}

// @Summary		删除自定义搜索引擎
// @Description	删除指定的自定义搜索引擎。
// @Tags			用户
// @Accept			json
// @Produce		json
// @Param			id	path		int	true	"搜索引擎ID"
// @Success		200	{object}	response.SuccessResponse	"删除成功"
// @Failure		401	{object}	response.ErrorResponse	"未授权"
// @Failure		403	{object}	response.ErrorResponse	"无权限"
// @Failure		404	{object}	response.ErrorResponse	"搜索引擎不存在"
// @Failure		500	{object}	response.ErrorResponse	"服务器内部错误"
// @Router			/user/search-engines/{id} [delete]
func (h *UserHandler) DeleteSearchEngine() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetInt64("user_id")
		engineIDStr := c.Param("id")
		engineID, err := strconv.ParseInt(engineIDStr, 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		// 检查搜索引擎是否属于当前用户
		engine, err := h.customSearchEngineService.GetSearchEngineByID(c.Request.Context(), engineID)
		if err != nil {
			response.Error(c, http.StatusNotFound, err)
			return
		}
		if engine.UserID != userID {
			response.Error(c, http.StatusForbidden, ErrNotOwner)
			return
		}

		err = h.customSearchEngineService.DeleteSearchEngine(c.Request.Context(), engineID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, nil)
	}
}

// 错误定义
var (
	ErrNoPermission = &NoPermissionError{}
	ErrNotOwner     = &NotOwnerError{}
)

type NoPermissionError struct{}

func (e *NoPermissionError) Error() string {
	return "you don't have permission to use custom search engines"
}

type NotOwnerError struct{}

func (e *NotOwnerError) Error() string {
	return "search engine does not belong to you"
}
