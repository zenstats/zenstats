// Package external 处理外部事件采集请求，接收前端 SDK 上报的埋点事件数据。
package external

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/common"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/pkg/globals"
	"github.com/zenstats/zenstats/pkg/utils"
)

// ingestLimiter 事件采集限频器，按站点域名维度限流。
type ingestLimiter struct {
	mu      sync.Mutex
	windows map[string]*ingestWindow
}

type ingestWindow struct {
	count     int
	resetTime time.Time
}

var limiter = &ingestLimiter{
	windows: make(map[string]*ingestWindow),
}

// allow 检查指定域名是否允许采集。当站点未配置限频（scale=0 或 limit=0）时不限流。
func (l *ingestLimiter) allow(domain string, scaleSeconds, limitPerMinute int) bool {
	if scaleSeconds <= 0 || limitPerMinute <= 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	key := fmt.Sprintf("%s:%d:%d", domain, scaleSeconds, limitPerMinute)
	w, exists := l.windows[key]
	if !exists || now.After(w.resetTime) {
		w = &ingestWindow{
			count:     1,
			resetTime: now.Add(time.Duration(scaleSeconds) * time.Second),
		}
		l.windows[key] = w
		return true
	}
	if w.count >= limitPerMinute {
		return false
	}
	w.count++
	return true
}

// cleanup 清理过期的限频窗口
func (l *ingestLimiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	for k, w := range l.windows {
		if now.After(w.resetTime) {
			delete(l.windows, k)
		}
	}
}

// setEventRequestDefaults 设置 EventRequest 的默认值
func setEventRequestDefaults(req *common.EventRequest, c *gin.Context) {
	req.Timestamp = time.Now()
	req.Ip = utils.ClientIP(c.Request)
	req.UserAgent = c.Request.UserAgent()
}

// parseInteractive 解析 Interactive 字段
func parseInteractive(req *common.EventRequest, tempReq *common.TempEventRequest) {
	req.Interactive = true
	if tempReq.Interactive != nil {
		var isFalse bool
		if err := json.Unmarshal(*tempReq.Interactive, &isFalse); err == nil && !isFalse {
			req.Interactive = false
		}
	}
}

// Event 返回事件采集的 gin 处理函数。
// 接收前端 SDK 上报的事件数据，验证域名合法性后将事件入队处理。
// 成功返回 HTTP 202 Accepted，失败返回对应错误码。
//
//	@Summary		上报埋点事件
//	@Description	接收前端 SDK 上报的事件数据，验证域名后入队处理
//	@Tags			事件采集
//	@Accept			json
//	@Produce		json
//	@Param			body	body		common.EventRequest	true	"事件数据"
//	@Success		202		{string}	string				"事件已接受"
//	@Failure		400		{string}	string				"请求参数错误或域名不允许"
//	@Failure		429		{string}	string				"请求频率超限"
//	@Router			/event [post]
func Event(siteService *service.SiteService) gin.HandlerFunc {
	return func(c *gin.Context) {
		slog.Debug("received event")

		var tempReq common.TempEventRequest
		body, err := c.GetRawData()
		if err != nil {
			slog.Error("failed to get raw data", "error", err)
			c.JSON(http.StatusBadRequest, "bad")
			return
		}
		slog.Debug("request body", "body", string(body))

		err = json.Unmarshal(body, &tempReq)
		if err != nil {
			slog.Error("failed to unmarshal request body", "error", err)
			c.JSON(http.StatusBadRequest, "bad")
			return
		}

		// Set default value for Interactive if not provided
		if tempReq.Interactive == nil {
			trueVal := json.RawMessage("true")
			tempReq.Interactive = &trueVal
		}

		can, err := siteService.IsDomainInList(c, tempReq.Domain)
		if err != nil {
			slog.Error("failed to check domain", "error", err)
			c.JSON(http.StatusBadRequest, "bad")
			return
		}
		if !can {
			slog.Warn("domain not allowed", "domain", tempReq.Domain)
			c.JSON(http.StatusBadRequest, "bad")
			return
		}

		// 限频检查：仅在站点配置了限频时生效
		site, err := siteService.GetSiteByDomain(c, tempReq.Domain)
		if err == nil {
			if !limiter.allow(tempReq.Domain, site.IngestRateLimitScaleSeconds, site.IngestLimitPerMinute) {
				c.JSON(http.StatusTooManyRequests, "rate limit exceeded")
				return
			}
		}
		// 定期清理过期窗口
		limiter.cleanup()

		req := common.EventRequest{
			Timestamp:      tempReq.Timestamp,
			Hash:           tempReq.Hash,
			EventName:      tempReq.EventName,
			JSVersion:      tempReq.JSVersion,
			URL:            tempReq.URL,
			Domain:         tempReq.Domain,
			Referrer:       tempReq.Referrer,
			Props:          tempReq.Props,
			EngagementTime: tempReq.EngagementTime,
			ScrollDepth:    tempReq.ScrollDepth,
		}

		setEventRequestDefaults(&req, c)
		parseInteractive(&req, &tempReq)

		queue := globals.GetQueue()
		if err = queue.Enqueue(&req); err != nil {
			c.JSON(http.StatusBadRequest, "error")
			return
		}

		c.JSON(http.StatusAccepted, "ok")
	}
}
