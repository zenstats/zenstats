// Package goals 处理目标管理相关的 HTTP 请求，包括目标的增删改查。
package goals

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/pkg/response"
)

// GoalsHandler 目标管理处理器。
type GoalsHandler struct {
	service *service.GoalService
}

// NewGoalsHandler 创建并返回一个新的 GoalsHandler 实例。
func NewGoalsHandler() *GoalsHandler {
	return &GoalsHandler{
		service: service.GetGoalService(),
	}
}

// List 获取站点的所有目标。
//
//	@Summary		获取目标列表
//	@Description	获取指定站点的所有转化目标。
//	@Tags			目标管理
//	@Security		BearerAuth
//	@Produce		json
//	@Param			domain		path		string	true	"站点域名"
//	@Success		200			{object}	response.SuccessResponse{data=[]service.Goal}	"成功响应"
//	@Failure		401			{object}	response.ErrorResponse	"未认证"
//	@Failure		500			{object}	response.ErrorResponse	"服务器内部错误"
//	@Router			/api/sites/{domain}/goals [get]
func (h *GoalsHandler) List() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteID, err := h.getSiteID(c)
		if err != nil {
			response.Error(c, http.StatusNotFound, err)
			return
		}

		goals, err := h.service.ListGoals(c, siteID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, goals)
	}
}

// Create 创建新目标。
//
//	@Summary		创建目标
//	@Description	为指定站点创建新的转化目标。event_name 和 page_path 必须提供其中一个。
//	@Tags			目标管理
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			domain		path		string						true	"站点域名"
//	@Param			body		body		service.CreateGoalRequest	true	"创建目标请求"
//	@Success		200			{object}	response.SuccessResponse{data=service.Goal}	"成功响应"
//	@Failure		400			{object}	response.ErrorResponse	"请求参数错误"
//	@Failure		401			{object}	response.ErrorResponse	"未认证"
//	@Failure		500			{object}	response.ErrorResponse	"服务器内部错误"
//	@Router			/api/sites/{domain}/goals [post]
func (h *GoalsHandler) Create() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteID, err := h.getSiteID(c)
		if err != nil {
			response.Error(c, http.StatusNotFound, err)
			return
		}

		var req service.CreateGoalRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		goal, err := h.service.CreateGoal(c, siteID, &req)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		response.Success(c, goal)
	}
}

// Update 更新目标。
//
//	@Summary		更新目标
//	@Description	更新指定目标的信息。
//	@Tags			目标管理
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			domain		path		string						true	"站点域名"
//	@Param			goalId		path		int							true	"目标ID"
//	@Param			body		body		service.UpdateGoalRequest	true	"更新目标请求"
//	@Success		200			{object}	response.SuccessResponse{data=service.Goal}	"成功响应"
//	@Failure		400			{object}	response.ErrorResponse	"请求参数错误"
//	@Failure		401			{object}	response.ErrorResponse	"未认证"
//	@Failure		404			{object}	response.ErrorResponse	"目标不存在"
//	@Failure		500			{object}	response.ErrorResponse	"服务器内部错误"
//	@Router			/api/sites/{domain}/goals/{goalId} [put]
func (h *GoalsHandler) Update() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteID, err := h.getSiteID(c)
		if err != nil {
			response.Error(c, http.StatusNotFound, err)
			return
		}

		goalID, err := strconv.ParseInt(c.Param("goalId"), 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		var req service.UpdateGoalRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		goal, err := h.service.UpdateGoal(c, siteID, goalID, &req)
		if err != nil {
			response.Error(c, http.StatusNotFound, err)
			return
		}

		response.Success(c, goal)
	}
}

// Delete 删除目标。
//
//	@Summary		删除目标
//	@Description	删除指定的目标。
//	@Tags			目标管理
//	@Security		BearerAuth
//	@Produce		json
//	@Param			domain		path		string	true	"站点域名"
//	@Param			goalId		path		int		true	"目标ID"
//	@Success		200			{object}	response.SuccessResponse	"成功响应"
//	@Failure		401			{object}	response.ErrorResponse	"未认证"
//	@Failure		404			{object}	response.ErrorResponse	"目标不存在"
//	@Failure		500			{object}	response.ErrorResponse	"服务器内部错误"
//	@Router			/api/sites/{domain}/goals/{goalId} [delete]
func (h *GoalsHandler) Delete() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteID, err := h.getSiteID(c)
		if err != nil {
			response.Error(c, http.StatusNotFound, err)
			return
		}

		goalID, err := strconv.ParseInt(c.Param("goalId"), 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		if err := h.service.DeleteGoal(c, siteID, goalID); err != nil {
			response.Error(c, http.StatusNotFound, err)
			return
		}

		response.Success(c, nil)
	}
}

// getSiteID 从上下文获取站点ID。
func (h *GoalsHandler) getSiteID(c *gin.Context) (int64, error) {
	domain := c.Param("domain")
	siteService := service.GetSiteService()
	site, err := siteService.GetSiteByDomain(c, domain)
	if err != nil {
		return 0, err
	}
	return site.ID, nil
}
