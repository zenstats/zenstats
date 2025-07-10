package sites

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/api/types"
	"github.com/zenstats/zenstats/pkg/response"
)

// ListShieldRuleIP 获取IP屏蔽规则列表
//
//	@Summary		获取IP屏蔽规则列表
//	@Description	获取指定域名的IP屏蔽规则列表
//	@Tags			站点管理
//	@Security		BearerAuth
//
//	@Accept			json
//	@Produce		json
//	@Param			domain	path		string											true	"站点域名"
//	@Success		200		{object}	response.SuccessResponse{data=[]interface{}}	"成功响应，返回IP屏蔽规则列表"
//	@Failure		400		{object}	response.ErrorResponse							"请求参数错误"
//	@Failure		500		{object}	response.ErrorResponse							"服务器内部错误"
//	@Router			/sites/{domain}/shield/ip [get]
func (h *SitesHandler) ListShieldRuleIP() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取domain
		domain := c.Param("domain")
		if domain == "" {
			response.Error(c, http.StatusBadRequest, errors.New("domain is required"))
			return
		}

		// 调用服务层获取IP规则列表
		rules, err := h.service.ListShieldRuleIP(c, domain)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		response.Success(c, rules)
	}
}

// AddShieldRuleIP 添加IP屏蔽规则
//
//	@Summary		添加IP屏蔽规则
//	@Description	为指定域名添加IP屏蔽规则
//	@Tags			站点管理
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			domain	path		string										true	"站点域名"
//	@Param			body	body		types.AddShieldRuleIPRequest				true	"添加IP屏蔽规则请求参数"
//	@Success		200		{object}	response.SuccessResponse{data=interface{}}	"成功响应，返回添加的IP屏蔽规则"
//	@Failure		400		{object}	response.ErrorResponse						"请求参数错误"
//	@Failure		500		{object}	response.ErrorResponse						"服务器内部错误"
//	@Router			/sites/{domain}/shield/ip [post]
func (h *SitesHandler) AddShieldRuleIP() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取domain
		domain := c.Param("domain")
		if domain == "" {
			response.Error(c, http.StatusBadRequest, errors.New("domain is required"))
			return
		}

		// 解析请求参数
		var req types.AddShieldRuleIPRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		// 调用服务层添加IP规则
		rule, err := h.service.AddShieldRuleIP(c, domain, req.IP, req.Action, req.Description)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		response.Success(c, rule)
	}
}

// RemoveShieldRuleIP 删除IP屏蔽规则
//
//	@Summary		删除IP屏蔽规则
//	@Description	删除指定域名的IP屏蔽规则
//	@Tags			站点管理
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			domain	path		string								true	"站点域名"
//	@Param			ruleId	path		int									true	"规则ID"
//	@Success		200		{object}	response.SuccessResponse{data=nil}	"成功响应"
//	@Failure		400		{object}	response.ErrorResponse				"请求参数错误"
//	@Failure		500		{object}	response.ErrorResponse				"服务器内部错误"
//	@Router			/sites/{domain}/shield/ip/{ruleId} [delete]
func (h *SitesHandler) RemoveShieldRuleIP() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取domain
		domain := c.Param("domain")
		if domain == "" {
			response.Error(c, http.StatusBadRequest, errors.New("domain is required"))
			return
		}

		// 获取规则ID
		ruleID, err := strconv.ParseInt(c.Param("ruleId"), 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		// 调用服务层删除IP规则
		if err := h.service.RemoveShieldRuleIP(c, domain, ruleID); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		response.Success(c, nil)
	}
}

// ListShieldRuleHostname 获取Hostname屏蔽规则列表
//
//	@Summary		获取Hostname屏蔽规则列表
//	@Description	获取指定域名的Hostname屏蔽规则列表
//	@Tags			站点管理
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			domain	path		string											true	"站点域名"
//	@Success		200		{object}	response.SuccessResponse{data=[]interface{}}	"成功响应，返回Hostname屏蔽规则列表"
//	@Failure		400		{object}	response.ErrorResponse							"请求参数错误"
//	@Failure		500		{object}	response.ErrorResponse							"服务器内部错误"
//	@Router			/sites/{domain}/shield/hostname [get]
func (h *SitesHandler) ListShieldRuleHostname() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取domain
		domain := c.Param("domain")
		if domain == "" {
			response.Error(c, http.StatusBadRequest, errors.New("domain is required"))
			return
		}
		// 调用服务层获取Hostname规则列表
		rules, err := h.service.ListShieldRuleHostname(c, domain)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		response.Success(c, rules)
	}
}

// AddShieldRuleHostname 添加Hostname屏蔽规则
//
//	@Summary		添加Hostname屏蔽规则
//	@Description	为指定域名添加Hostname屏蔽规则
//	@Tags			站点管理
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			domain	path		string										true	"站点域名"
//	@Param			body	body		types.AddShieldRuleHostnameRequest			true	"添加Hostname屏蔽规则请求参数"
//	@Success		200		{object}	response.SuccessResponse{data=interface{}}	"成功响应"
//	@Failure		400		{object}	response.ErrorResponse						"请求参数错误"
//	@Router			/sites/{domain}/shield/hostname [post]
func (h *SitesHandler) AddShieldRuleHostname() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取site ID
		domain := c.Param("domain")
		if domain == "" {
			response.Error(c, http.StatusBadRequest, errors.New("domain is required"))
			return
		}

		// 解析请求参数
		var req types.AddShieldRuleHostnameRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		// 调用服务层添加Hostname规则
		rule, err := h.service.AddShieldRuleHostname(c, domain, req.Hostname, req.HostnamePattern, req.Action)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		response.Success(c, rule)
	}
}

// RemoveShieldRuleHostname 删除Hostname屏蔽规则
//
//	@Summary		删除Hostname屏蔽规则
//	@Description	删除指定域名的Hostname屏蔽规则
//	@Tags			站点管理
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			domain	path		string								true	"站点域名"
//	@Param			ruleId	path		int									true	"规则ID"
//	@Success		200		{object}	response.SuccessResponse{data=nil}	"成功响应"
//	@Failure		400		{object}	response.ErrorResponse				"请求参数错误"
//	@Failure		500		{object}	response.ErrorResponse				"服务器内部错误"
//	@Router			/sites/{domain}/shield/hostname/{ruleId} [delete]
func (h *SitesHandler) RemoveShieldRuleHostname() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取domain
		domain := c.Param("domain")
		if domain == "" {
			response.Error(c, http.StatusBadRequest, errors.New("domain is required"))
			return
		}

		// 获取规则ID
		ruleID, err := strconv.ParseInt(c.Param("ruleId"), 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		// 调用服务层删除Hostname规则
		if err := h.service.RemoveShieldRuleHostname(c, domain, ruleID); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		response.Success(c, nil)
	}
}

// ListShieldRuleCountry 获取Country屏蔽规则列表
//
//	@Summary		获取Country屏蔽规则列表
//	@Description	获取指定域名的Country屏蔽规则列表
//	@Tags			站点管理
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			domain	path		string											true	"站点域名"
//	@Success		200		{object}	response.SuccessResponse{data=[]interface{}}	"成功响应，返回Country屏蔽规则列表"
//	@Failure		400		{object}	response.ErrorResponse							"请求参数错误"
//	@Failure		500		{object}	response.ErrorResponse							"服务器内部错误"
//	@Router			/sites/{domain}/shield/country [get]
func (h *SitesHandler) ListShieldRuleCountry() gin.HandlerFunc {
	return func(c *gin.Context) {
		domain := c.Param("domain")
		if domain == "" {
			response.Error(c, http.StatusBadRequest, errors.New("domain is required"))
			return
		}
		// 调用服务层获取Country规则列表
		rules, err := h.service.ListShieldRuleCountry(c, domain)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		response.Success(c, rules)
	}
}

// AddShieldRuleCountry 添加Country屏蔽规则
//
//	@Summary		添加Country屏蔽规则
//	@Description	为指定域名添加Country屏蔽规则
//	@Tags			站点管理
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			domain	path		string										true	"站点域名"
//	@Param			body	body		types.AddShieldRuleCountryRequest			true	"添加Country屏蔽规则请求参数"
//	@Success		200		{object}	response.SuccessResponse{data=interface{}}	"成功响应，返回添加的Country屏蔽规则"
//	@Failure		400		{object}	response.ErrorResponse						"请求参数错误"
//	@Failure		500		{object}	response.ErrorResponse						"服务器内部错误"
//	@Router			/sites/{domain}/shield/country [post]
func (h *SitesHandler) AddShieldRuleCountry() gin.HandlerFunc {
	return func(c *gin.Context) {
		domain := c.Param("domain")
		if domain == "" {
			response.Error(c, http.StatusBadRequest, errors.New("domain is required"))
			return
		}

		// 解析请求参数
		var req types.AddShieldRuleCountryRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		// 添加Country规则
		rule, err := h.service.AddShieldRuleCountry(c, domain, req.CountryCode, req.Action)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		response.Success(c, rule)
	}
}

// RemoveShieldRuleCountry 删除Country屏蔽规则
//
//	@Summary		删除Country屏蔽规则
//	@Description	删除指定站点的Country屏蔽规则
//	@Tags			站点管理
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int									true	"站点ID"
//	@Param			ruleId	path		int									true	"规则ID"
//	@Success		200		{object}	response.SuccessResponse{data=nil}	"成功响应"
//	@Failure		400		{object}	response.ErrorResponse				"请求参数错误"
//	@Failure		500		{object}	response.ErrorResponse				"服务器内部错误"
//	@Router			/sites/{id}/shield/country/{ruleId} [delete]
func (h *SitesHandler) RemoveShieldRuleCountry() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取site ID
		siteID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		// 获取规则ID
		ruleID, err := strconv.ParseInt(c.Param("ruleId"), 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		// 调用服务层删除Country规则
		if err := h.service.RemoveShieldRuleCountry(c, siteID, ruleID); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		response.Success(c, nil)
	}
}
