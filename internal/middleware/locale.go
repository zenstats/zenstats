package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/pkg/i18n"
)

const LocaleKey = "locale"

func LocaleMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		locale := i18n.GetLocale(c)
		c.Set(LocaleKey, locale)
		c.Next()
	}
}

func GetLocale(c *gin.Context) string {
	if locale, exists := c.Get(LocaleKey); exists {
		return locale.(string)
	}
	return "en"
}
