package middleware

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/config"
	"github.com/zenstats/zenstats/pkg/iputil"
)

// AuthRateLimit limits attempts across the routes using the returned handler.
func AuthRateLimit() gin.HandlerFunc {
	return RateLimit(config.Conf.RateLimit.AuthPerMinute, time.Minute, 10000)
}

// RateLimit uses bounded fixed windows. At capacity, new IPs fail closed until
// an entry expires, rather than evicting live counters and enabling a bypass.
func RateLimit(limit int, window time.Duration, maxEntries int) gin.HandlerFunc {
	return rateLimit(limit, window, maxEntries, time.Now)
}

func rateLimit(limit int, window time.Duration, maxEntries int, now func() time.Time) gin.HandlerFunc {
	type entry struct {
		count int
		until time.Time
	}
	var mu sync.Mutex
	entries := make(map[string]entry)
	if window <= 0 {
		window = time.Minute
	}
	if maxEntries <= 0 {
		maxEntries = 10000
	}
	return func(c *gin.Context) {
		if limit <= 0 {
			c.Next()
			return
		}
		key := iputil.ClientIP(c.Request)
		current := now()
		mu.Lock()
		e, exists := entries[key]
		if exists && !current.Before(e.until) {
			delete(entries, key)
			exists = false
		}
		retry := window
		allowed := false
		if !exists {
			if len(entries) >= maxEntries {
				for k, item := range entries {
					if !current.Before(item.until) {
						delete(entries, k)
					} else if d := item.until.Sub(current); d < retry {
						retry = d
					}
				}
			}
			if len(entries) < maxEntries {
				e = entry{until: current.Add(window)}
				exists = true
			}
		}
		if exists {
			retry = e.until.Sub(current)
			if e.count < limit {
				e.count++
				entries[key] = e
				allowed = true
			}
		}
		mu.Unlock()
		if !allowed {
			c.Header("Retry-After", strconv.Itoa(max(1, int(math.Ceil(retry.Seconds())))))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code": 429, "message": "too many requests", "error": "rate limit exceeded, please retry later",
			})
			return
		}
		c.Next()
	}
}
