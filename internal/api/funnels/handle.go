// Package funnels 处理漏斗管理相关的 HTTP 请求，包括漏斗的增删改查和漏斗分析。
package funnels

import (
	"net/http"
	"strconv"
	"time"

	"github.com/dromara/carbon/v2"
	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/internal/service/funnel"
	"github.com/zenstats/zenstats/pkg/response"
)

// FunnelsHandler 漏斗管理处理器。
type FunnelsHandler struct {
	funnelService   *service.FunnelService
	goalService     *service.GoalService
	analysisService *funnel.AnalysisService
}

// NewFunnelsHandler 创建并返回一个新的 FunnelsHandler 实例。
func NewFunnelsHandler() *FunnelsHandler {
	return &FunnelsHandler{
		funnelService:   service.GetFunnelService(),
		goalService:     service.GetGoalService(),
		analysisService: funnel.NewAnalysisService(),
	}
}

// List 获取站点的所有漏斗。
//
//	@Summary		获取漏斗列表
//	@Description	获取指定站点的所有漏斗。
//	@Tags			漏斗管理
//	@Security		BearerAuth
//	@Produce		json
//	@Param			domain		path		string	true	"站点域名"
//	@Success		200			{object}	response.SuccessResponse{data=[]service.Funnel}	"成功响应"
//	@Failure		401			{object}	response.ErrorResponse	"未认证"
//	@Failure		500			{object}	response.ErrorResponse	"服务器内部错误"
//	@Router			/api/sites/{domain}/funnels [get]
func (h *FunnelsHandler) List() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteID, err := h.getSiteID(c)
		if err != nil {
			response.Error(c, http.StatusNotFound, err)
			return
		}

		funnels, err := h.funnelService.ListFunnels(c, siteID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, funnels)
	}
}

// Get 获取单个漏斗详情。
//
//	@Summary		获取漏斗详情
//	@Description	获取指定漏斗的详细信息，包含步骤的目标信息。
//	@Tags			漏斗管理
//	@Security		BearerAuth
//	@Produce		json
//	@Param			domain		path		string	true	"站点域名"
//	@Param			funnelId	path		int		true	"漏斗ID"
//	@Success		200			{object}	response.SuccessResponse{data=service.FunnelDetail}	"成功响应"
//	@Failure		401			{object}	response.ErrorResponse	"未认证"
//	@Failure		404			{object}	response.ErrorResponse	"漏斗不存在"
//	@Failure		500			{object}	response.ErrorResponse	"服务器内部错误"
//	@Router			/api/sites/{domain}/funnels/{funnelId} [get]
func (h *FunnelsHandler) Get() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteID, err := h.getSiteID(c)
		if err != nil {
			response.Error(c, http.StatusNotFound, err)
			return
		}

		funnelID, err := strconv.ParseInt(c.Param("funnelId"), 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		f, err := h.funnelService.GetFunnel(c, siteID, funnelID)
		if err != nil {
			response.Error(c, http.StatusNotFound, err)
			return
		}

		response.Success(c, f)
	}
}

// Create 创建新漏斗。
//
//	@Summary		创建漏斗
//	@Description	为指定站点创建新的转化漏斗。
//	@Tags			漏斗管理
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			domain		path		string						true	"站点域名"
//	@Param			body		body		service.CreateFunnelRequest	true	"创建漏斗请求"
//	@Success		200			{object}	response.SuccessResponse{data=service.Funnel}	"成功响应"
//	@Failure		400			{object}	response.ErrorResponse	"请求参数错误"
//	@Failure		401			{object}	response.ErrorResponse	"未认证"
//	@Failure		500			{object}	response.ErrorResponse	"服务器内部错误"
//	@Router			/api/sites/{domain}/funnels [post]
func (h *FunnelsHandler) Create() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteID, err := h.getSiteID(c)
		if err != nil {
			response.Error(c, http.StatusNotFound, err)
			return
		}

		var req service.CreateFunnelRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		f, err := h.funnelService.CreateFunnel(c, siteID, &req)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		response.Success(c, f)
	}
}

// Update 更新漏斗。
//
//	@Summary		更新漏斗
//	@Description	更新指定漏斗的信息和步骤。
//	@Tags			漏斗管理
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			domain		path		string						true	"站点域名"
//	@Param			funnelId	path		int							true	"漏斗ID"
//	@Param			body		body		service.UpdateFunnelRequest	true	"更新漏斗请求"
//	@Success		200			{object}	response.SuccessResponse{data=service.FunnelDetail}	"成功响应"
//	@Failure		400			{object}	response.ErrorResponse	"请求参数错误"
//	@Failure		401			{object}	response.ErrorResponse	"未认证"
//	@Failure		404			{object}	response.ErrorResponse	"漏斗不存在"
//	@Failure		500			{object}	response.ErrorResponse	"服务器内部错误"
//	@Router			/api/sites/{domain}/funnels/{funnelId} [put]
func (h *FunnelsHandler) Update() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteID, err := h.getSiteID(c)
		if err != nil {
			response.Error(c, http.StatusNotFound, err)
			return
		}

		funnelID, err := strconv.ParseInt(c.Param("funnelId"), 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		var req service.UpdateFunnelRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		f, err := h.funnelService.UpdateFunnel(c, siteID, funnelID, &req)
		if err != nil {
			response.Error(c, http.StatusNotFound, err)
			return
		}

		response.Success(c, f)
	}
}

// Delete 删除漏斗。
//
//	@Summary		删除漏斗
//	@Description	删除指定的漏斗。
//	@Tags			漏斗管理
//	@Security		BearerAuth
//	@Produce		json
//	@Param			domain		path		string	true	"站点域名"
//	@Param			funnelId	path		int		true	"漏斗ID"
//	@Success		200			{object}	response.SuccessResponse	"成功响应"
//	@Failure		401			{object}	response.ErrorResponse	"未认证"
//	@Failure		404			{object}	response.ErrorResponse	"漏斗不存在"
//	@Failure		500			{object}	response.ErrorResponse	"服务器内部错误"
//	@Router			/api/sites/{domain}/funnels/{funnelId} [delete]
func (h *FunnelsHandler) Delete() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteID, err := h.getSiteID(c)
		if err != nil {
			response.Error(c, http.StatusNotFound, err)
			return
		}

		funnelID, err := strconv.ParseInt(c.Param("funnelId"), 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		if err := h.funnelService.DeleteFunnel(c, siteID, funnelID); err != nil {
			response.Error(c, http.StatusNotFound, err)
			return
		}

		response.Success(c, nil)
	}
}

// Analyze 执行漏斗分析。
//
//	@Summary		漏斗分析
//	@Description	分析指定漏斗在给定时间范围内的转化数据。
//	@Tags			漏斗分析
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			domain		path		string	true	"站点域名"
//	@Param			funnelId	path		int		true	"漏斗ID"
//	@Param			period		query		string	true	"时间周期" Enums(realtime, day, p7, p14, p30, custom)
//	@Param			date		query		string	false	"统计日期，格式 YYYY-MM-DD"
//	@Param			from		query		string	false	"自定义开始日期，period=custom 时必填"
//	@Param			to			query		string	false	"自定义结束日期，period=custom 时必填"
//	@Success		200			{object}	response.SuccessResponse{data=funnel.FunnelAnalysisResult}	"成功响应"
//	@Failure		400			{object}	response.ErrorResponse	"请求参数错误"
//	@Failure		401			{object}	response.ErrorResponse	"未认证"
//	@Failure		404			{object}	response.ErrorResponse	"漏斗不存在"
//	@Failure		500			{object}	response.ErrorResponse	"服务器内部错误"
//	@Router			/api/stats/{domain}/funnel/{funnelId} [get]
func (h *FunnelsHandler) Analyze() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteID, err := h.getSiteID(c)
		if err != nil {
			response.Error(c, http.StatusNotFound, err)
			return
		}

		funnelID, err := strconv.ParseInt(c.Param("funnelId"), 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		// 获取漏斗详情
		f, err := h.funnelService.GetFunnel(c, siteID, funnelID)
		if err != nil {
			response.Error(c, http.StatusNotFound, err)
			return
		}

		// 解析时间范围
		start, end, err := h.parseTimeRange(c)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		// 构建漏斗步骤
		steps := make([]*funnel.FunnelStep, len(f.Steps))
		for i, step := range f.Steps {
			steps[i] = &funnel.FunnelStep{
				GoalID:    step.GoalID,
				GoalType:  step.GoalType,
				GoalValue: step.GoalValue,
				GoalName:  step.GoalName,
				StepOrder: step.StepOrder,
			}
		}

		// 执行分析
		result, err := h.analysisService.Analyze(c, &funnel.AnalysisRequest{
			SiteID:    strconv.FormatInt(siteID, 10),
			Steps:     steps,
			StartTime: start,
			EndTime:   end,
		})
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, result)
	}
}

// getSiteID 从上下文获取站点ID。
func (h *FunnelsHandler) getSiteID(c *gin.Context) (int64, error) {
	domain := c.Param("domain")
	siteService := service.GetSiteService()
	site, err := siteService.GetSiteByDomain(c, domain)
	if err != nil {
		return 0, err
	}
	return site.ID, nil
}

// parseTimeRange 解析时间范围参数。
func (h *FunnelsHandler) parseTimeRange(c *gin.Context) (time.Time, time.Time, error) {
	period := c.DefaultQuery("period", "p30")
	date := c.Query("date")
	from := c.Query("from")
	to := c.Query("to")

	siteService := service.GetSiteService()
	domain := c.Param("domain")
	site, err := siteService.GetSiteByDomain(c, domain)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	timezone := site.Timezone
	if timezone == "" {
		timezone = "UTC"
	}

	var startDate, endDate time.Time

	switch period {
	case "realtime":
		endDate = carbon.Now(timezone).SetTimezone(carbon.UTC).StdTime()
		startDate = carbon.Now(timezone).SubMinutes(30).SetTimezone(carbon.UTC).StdTime()
	case "day", "yesterday":
		if date == "" && period == "yesterday" {
			date = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
		} else if date == "" {
			date = time.Now().Format("2006-01-02")
		}
		d := carbon.Parse(date, timezone)
		startDate = d.StartOfDay().SetTimezone(carbon.UTC).StdTime()
		endDate = d.EndOfDay().SetTimezone(carbon.UTC).StdTime()
	case "p7":
		if date == "" {
			date = time.Now().Format("2006-01-02")
		}
		baseDate := carbon.Parse(date, timezone)
		endDate = baseDate.EndOfDay().SetTimezone(carbon.UTC).StdTime()
		startDate = baseDate.SubDays(6).StartOfDay().SetTimezone(carbon.UTC).StdTime()
	case "p14":
		if date == "" {
			date = time.Now().Format("2006-01-02")
		}
		baseDate := carbon.Parse(date, timezone)
		endDate = baseDate.EndOfDay().SetTimezone(carbon.UTC).StdTime()
		startDate = baseDate.SubDays(13).StartOfDay().SetTimezone(carbon.UTC).StdTime()
	case "p30":
		if date == "" {
			date = time.Now().Format("2006-01-02")
		}
		baseDate := carbon.Parse(date, timezone)
		endDate = baseDate.EndOfDay().SetTimezone(carbon.UTC).StdTime()
		startDate = baseDate.SubDays(29).StartOfDay().SetTimezone(carbon.UTC).StdTime()
	case "custom":
		if from == "" || to == "" {
			return time.Time{}, time.Time{}, http.ErrMissingFile
		}
		startDate = carbon.Parse(from, timezone).StartOfDay().SetTimezone(carbon.UTC).StdTime()
		endDate = carbon.Parse(to, timezone).EndOfDay().SetTimezone(carbon.UTC).StdTime()
	default:
		if date == "" {
			date = time.Now().Format("2006-01-02")
		}
		baseDate := carbon.Parse(date, timezone)
		endDate = baseDate.EndOfDay().SetTimezone(carbon.UTC).StdTime()
		startDate = baseDate.SubDays(29).StartOfDay().SetTimezone(carbon.UTC).StdTime()
	}

	return startDate, endDate, nil
}
