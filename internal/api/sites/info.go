package sites

import "github.com/gin-gonic/gin"

func (h *SitesHandler) Info() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	}
}
