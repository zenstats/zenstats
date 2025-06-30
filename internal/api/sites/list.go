package sites

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/pkg/response"
)

func (h *SitesHandler) List() gin.HandlerFunc {
	return func(c *gin.Context) {
		domain := c.Query("domain")
		list, err := h.service.GetUserSiteByDomain(c, domain)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}
		response.Success(c, list)
	}
}
