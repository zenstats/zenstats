package i18n

import (
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

var (
	mu           sync.RWMutex
	translations = map[string]map[string]string{
		"en":    enTranslations,
		"zh-CN": zhCNTranslations,
	}
)

// T returns the translated string for the given locale and key.
// Falls back to English if key not found.
func T(locale, key string) string {
	mu.RLock()
	defer mu.RUnlock()

	lang := normalizeLocale(locale)
	if msgs, ok := translations[lang]; ok {
		if msg, ok := msgs[key]; ok {
			return msg
		}
	}
	// Fallback to English
	if msgs, ok := translations["en"]; ok {
		if msg, ok := msgs[key]; ok {
			return msg
		}
	}
	return key
}

// GetLocale extracts the locale from the gin context.
// Checks Accept-Language header first, then ?lang= query param.
func GetLocale(c *gin.Context) string {
	// Check query param first
	if lang := c.Query("lang"); lang != "" {
		return normalizeLocale(lang)
	}

	// Check Accept-Language header
	acceptLang := c.GetHeader("Accept-Language")
	if acceptLang == "" {
		return "en"
	}

	// Parse Accept-Language: "zh-CN,zh;q=0.9,en;q=0.8" -> "zh-CN"
	langs := strings.Split(acceptLang, ",")
	for _, l := range langs {
		parts := strings.SplitN(l, ";", 2)
		lang := strings.TrimSpace(parts[0])
		if lang != "" {
			return normalizeLocale(lang)
		}
	}

	return "en"
}

func normalizeLocale(locale string) string {
	locale = strings.TrimSpace(locale)
	locale = strings.ReplaceAll(locale, "_", "-")
	// Handle "zh" -> "zh-CN"
	if locale == "zh" {
		return "zh-CN"
	}
	// Handle "zh-Hans" -> "zh-CN"
	if strings.HasPrefix(locale, "zh") {
		return "zh-CN"
	}
	return locale
}
