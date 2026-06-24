package sites

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	"github.com/zenstats/zenstats/pkg/response"
)

// ============================================================================
// 垃圾 Referrer 规则 CRUD
// ============================================================================

// ListShieldRuleReferrer GET /api/sites/:domain/shield/referrer
func (h *SitesHandler) ListShieldRuleReferrer() gin.HandlerFunc {
	return func(c *gin.Context) {
		domain := c.Param("domain")
		site, err := h.service.GetVerifiedSiteByDomain(c, domain)
		if err != nil {
			response.Error(c, http.StatusNotFound, fmt.Errorf("site not found"))
			return
		}

		rules, err := h.service.ListShieldRuleReferrer(c, site.ID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		if rules == nil {
			rules = []*ent.ShieldRulesReferrer{}
		}
		response.Success(c, rules)
	}
}

// AddShieldRuleReferrer POST /api/sites/:domain/shield/referrer
func (h *SitesHandler) AddShieldRuleReferrer() gin.HandlerFunc {
	return func(c *gin.Context) {
		domain := c.Param("domain")
		site, err := h.service.GetVerifiedSiteByDomain(c, domain)
		if err != nil {
			response.Error(c, http.StatusNotFound, fmt.Errorf("site not found"))
			return
		}

		var req struct {
			Hostname    string `json:"hostname" binding:"required"`
			Action      string `json:"action"`
			Description string `json:"description"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}
		if req.Action == "" {
			req.Action = "deny"
		}

		rule, err := h.service.AddShieldRuleReferrer(c, site.ID, req.Hostname, req.Action, req.Description)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, rule)
	}
}

// RemoveShieldRuleReferrer DELETE /api/sites/:domain/shield/referrer/:ruleId
func (h *SitesHandler) RemoveShieldRuleReferrer() gin.HandlerFunc {
	return func(c *gin.Context) {
		domain := c.Param("domain")
		ruleIDStr := c.Param("ruleId")
		ruleID, err := strconv.ParseInt(ruleIDStr, 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, fmt.Errorf("invalid rule id"))
			return
		}

		site, err := h.service.GetVerifiedSiteByDomain(c, domain)
		if err != nil {
			response.Error(c, http.StatusNotFound, fmt.Errorf("site not found"))
			return
		}

		if err := h.service.RemoveShieldRuleReferrer(c, site.ID, ruleID); err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, gin.H{"ok": true})
	}
}
