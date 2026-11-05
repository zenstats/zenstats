// Package external 处理外部事件采集请求，接收前端 SDK 上报的埋点事件数据。
package external

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/event"
	"github.com/zenstats/zenstats/internal/ingestion"
	"github.com/zenstats/zenstats/internal/model"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/pkg/globals"
	"github.com/zenstats/zenstats/pkg/iputil"
)

// ingestLimiter 事件采集限频器，按站点域名维度限流。
type ingestLimiter struct {
	mu      sync.RWMutex
	windows map[string]*ingestWindow
}

type ingestWindow struct {
	count     int
	resetTime time.Time
}

var limiter = newIngestLimiter()

func newIngestLimiter() *ingestLimiter {
	l := &ingestLimiter{
		windows: make(map[string]*ingestWindow),
	}
	go l.cleanupLoop()
	return l
}

// cleanupLoop 每 5 分钟清理一次过期窗口，避免每次请求都遍历 map。
func (l *ingestLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		l.cleanup()
	}
}

// allow 检查指定域名是否允许采集。当站点未配置限频（scale=0 或 limit=0）时不限流。
func (l *ingestLimiter) allow(domain string, scaleSeconds, limitPerMinute int) bool {
	if scaleSeconds <= 0 || limitPerMinute <= 0 {
		return true
	}

	now := time.Now()
	key := fmt.Sprintf("%s:%d:%d", domain, scaleSeconds, limitPerMinute)

	// 先用读锁快速检查是否存在且未过期
	l.mu.RLock()
	w, exists := l.windows[key]
	l.mu.RUnlock()

	if exists && now.Before(w.resetTime) {
		// 窗口未过期，用写锁更新计数
		l.mu.Lock()
		defer l.mu.Unlock()
		if w.count >= limitPerMinute {
			return false
		}
		w.count++
		return true
	}

	// 窗口不存在或已过期，用写锁创建/重置窗口
	l.mu.Lock()
	defer l.mu.Unlock()
	// double-check：其他 goroutine 可能已更新
	if w, exists = l.windows[key]; exists && now.Before(w.resetTime) {
		if w.count >= limitPerMinute {
			return false
		}
		w.count++
		return true
	}
	l.windows[key] = &ingestWindow{
		count:     1,
		resetTime: now.Add(time.Duration(scaleSeconds) * time.Second),
	}
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

// verifyRequestOrigin Verify request origin是否匹配域名。
// allowedOrigins 是管理员配置的额外允许来源列表（逗号分隔），支持通配符（如 *.example.com）。
// 如果 Origin 和 Referer 都不存在（非浏览器客户端），放行。
func verifyRequestOrigin(c *gin.Context, domain string, allowedOrigins string) bool {
	origin := c.GetHeader("Origin")
	referer := c.GetHeader("Referer")

	// 两个头部都不存在 → 非浏览器客户端（如服务端 SDK），放行
	if origin == "" && referer == "" {
		return true
	}

	// 只解析一次 host，避免重复扫描
	originHost := extractHost(origin)
	refererHost := extractHost(referer)

	// 检查是否匹配站点自身域名
	if matchOriginHost(originHost, domain) || matchOriginHost(refererHost, domain) {
		return true
	}

	// 检查管理员配置的额外允许来源
	if allowedOrigins != "" {
		for _, extra := range strings.Split(allowedOrigins, ",") {
			extra = strings.TrimSpace(extra)
			if extra == "" {
				continue
			}
			if matchOriginHost(originHost, extra) || matchOriginHost(refererHost, extra) {
				return true
			}
		}
	}

	return false
}

// extractHost 从 URL/Origin 字符串中提取主机部分（host），不含端口号和路径。
// 支持带协议前缀（https://example.com）和裸主机名（example.com）两种格式。
// 热路径优化：避免 range/数组分配，直接用 if-else 剥离协议前缀。
func extractHost(raw string) string {
	if raw == "" {
		return ""
	}

	rest := raw

	// 直接比较协议前缀，避免 range + 数组字面量分配
	if len(raw) >= 8 && raw[:8] == "https://" {
		rest = raw[8:]
	} else if len(raw) >= 7 && raw[:7] == "http://" {
		rest = raw[7:]
	}

	// 手动查找主机结束位置（遇到 / : ? # 即停止）
	hostEnd := 0
	for hostEnd < len(rest) {
		c := rest[hostEnd]
		if c == '/' || c == ':' || c == '?' || c == '#' {
			break
		}
		hostEnd++
	}

	return rest[:hostEnd]
}

// matchOriginHost 检查主机名是否匹配域名模式。
// pattern 支持两种格式：
//   - 精确匹配：example.com、www.example.com
//   - 通配符匹配：*.example.com（匹配所有子域名）
//
// 热路径优化：直接字节比较替代 strings.HasPrefix，消除函数调用。
func matchOriginHost(host, pattern string) bool {
	if host == "" || pattern == "" {
		return false
	}

	// 直接检查 * 前缀，避免 strings.HasPrefix 函数调用
	allowSub := len(pattern) >= 2 && pattern[0] == '*' && pattern[1] == '.'
	baseDomain := pattern
	if allowSub {
		baseDomain = pattern[2:]
	}

	// 精确匹配：长度不等的字符串在 Go 中 O(1) 快速返回
	if host == baseDomain {
		return true
	}

	// 通配符子域名匹配：sub.example.com 匹配 *.example.com
	// host 必须比 baseDomain 长至少 2 字符（1 子域名标签 + 1 点号）
	if allowSub && len(host) > len(baseDomain) &&
		host[len(host)-len(baseDomain)-1] == '.' &&
		host[len(host)-len(baseDomain):] == baseDomain {
		return true
	}

	return false
}

// isDatacenterOrThreatIP 检查请求是否来自数据中心或已知威胁 IP。
// 依赖反向代理（如 Cloudflare）设置的 X-Zenstats-Ip-Type 请求头：
//   - dc_ip：数据中心/云服务商 IP（阿里云、腾讯云、AWS 等），应丢弃
//   - threat_ip：已知威胁 IP，应丢弃
func isDatacenterOrThreatIP(c *gin.Context) bool {
	ipType := strings.ToLower(c.GetHeader("X-Zenstats-Ip-Type"))
	if ipType == "" {
		ipType = strings.ToLower(c.GetHeader("Cf-Ip-Type"))
	}
	return ipType == "dc_ip" || ipType == "threat_ip"
}

// setEventRequestDefaults 设置 EventRequest 的默认值。
// 若客户端提供了合理的时间戳（如 batch 事件在客户端缓存时的真实时间），则保留；
// 否则回退到服务端接收时间。
func setEventRequestDefaults(req *model.EventRequest, c *gin.Context) {
	now := time.Now()
	// 仅接受非零且不早于 1 小时前、不晚于当前的客户端时间戳，防止时钟漂移/伪造
	if req.Timestamp.IsZero() || req.Timestamp.After(now) || req.Timestamp.Before(now.Add(-time.Hour)) {
		req.Timestamp = now
	}
	req.Ip = iputil.ClientIP(c.Request)
	req.UserAgent = c.Request.UserAgent()
}

// parseInteractive 解析 Interactive 字段
func parseInteractive(req *model.EventRequest, tempReq *model.TempEventRequest) {
	req.Interactive = true
	if tempReq.Interactive != nil {
		var isFalse bool
		if err := json.Unmarshal(*tempReq.Interactive, &isFalse); err == nil && !isFalse {
			req.Interactive = false
		}
	}
}

// newEventRequest is shared by single and batch ingestion so both retain the
// same fields, limits and defaults (especially revenue and interactive).
func newEventRequest(temp model.TempEventRequest, domain string, c *gin.Context) *model.EventRequest {
	req := &model.EventRequest{
		Timestamp: temp.Timestamp, Hash: temp.Hash, EventName: temp.EventName,
		JSVersion: temp.JSVersion, URL: temp.URL, Domain: domain,
		Referrer: temp.Referrer, Props: temp.Props,
		EngagementTime: temp.EngagementTime, ScrollDepth: temp.ScrollDepth,
		Revenue: temp.Revenue,
	}
	if len(req.EventName) > 120 {
		req.EventName = req.EventName[:120]
	}
	if len(req.URL) > 2000 {
		req.URL = req.URL[:2000]
	}
	setEventRequestDefaults(req, c)
	parseInteractive(req, &temp)
	return req
}

// Event 返回事件采集的 gin 处理函数。
// 接收前端 SDK 上报的事件数据，验证域名合法性后将事件入队处理。
// 成功返回 HTTP 202 Accepted，失败返回对应错误码。
//
//	@Summary		上报埋点事件
//	@Description	接收前端 SDK 上报的事件数据，验证域名后入队处理。字段使用短 key：n=事件名，u=页面 URL，d=站点域名，r=来源，p=自定义属性，e=停留时长，sd=滚动深度，i=是否交互。
//	@Tags			事件采集
//	@Accept			json
//	@Produce		json
//	@Param			body	body		model.EventRequest	true	"事件数据"
//	@Success		202		{string}	string				"ok"
//	@Failure		400		{string}	string				"请求参数错误或域名不允许"
//	@Failure		429		{string}	string				"请求频率超限"
//	@Failure		413		{string}	string				"请求体超过 1 MiB"
//	@Failure		503		{string}	string				"队列或站点查询暂不可用"
//	@Header			202		{integer}	X-Zenstats-Accepted	"已入内存队列事件数，不代表落库成功"
//	@Header			202		{integer}	X-Zenstats-Rejected	"校验拒绝事件数；混合批次不应整批重试"
//	@Header			503		{integer}	Retry-After		"建议重试等待秒数"
//	@Router			/event [post]
func Event(siteService *service.SiteService) gin.HandlerFunc {
	return eventHandler(siteService, ingestion.Default)
}

func eventHandler(siteService *service.SiteService, monitor *ingestion.Monitor) gin.HandlerFunc {
	return func(c *gin.Context) {
		monitor.Count("requests", 1)
		// Bound allocation before decoding or constructing a batch of events.
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
		body, err := c.GetRawData()
		if err != nil {
			status, reason := http.StatusBadRequest, "body_read_failed"
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				status, reason = http.StatusRequestEntityTooLarge, "body_too_large"
			}
			monitor.Outcome("request", "invalid_requests", reason, 1)
			c.JSON(status, "invalid request body")
			return
		}
		var probe struct {
			EventName string `json:"n"`
		}
		if err := json.Unmarshal(body, &probe); err != nil {
			monitor.Outcome("request", "invalid_requests", "invalid_json", 1)
			c.JSON(http.StatusBadRequest, "bad")
			return
		}
		isBatch := probe.EventName == "batch"
		var events []model.TempEventRequest
		if isBatch {
			var batch model.BatchEventRequest
			err = json.Unmarshal(body, &batch)
			events = batch.Events
			for i := range events {
				if events[i].Domain == "" {
					events[i].Domain = batch.Domain
				}
				if events[i].JSVersion == "" {
					events[i].JSVersion = batch.JSVersion
				}
			}
		} else {
			var single model.TempEventRequest
			err = json.Unmarshal(body, &single)
			events = []model.TempEventRequest{single}
		}
		if err != nil {
			monitor.Outcome("request", "invalid_requests", "invalid_payload", 1)
			c.JSON(http.StatusBadRequest, "bad")
			return
		}
		if len(events) == 0 {
			monitor.Outcome("request", "invalid_requests", "empty_batch", 1)
			c.JSON(http.StatusBadRequest, "empty batch")
			return
		}
		monitor.Count("received", len(events))
		if len(events) > 100 {
			monitor.Outcome("ingress", "rejected", "batch_too_large", len(events))
			c.Header("X-Zenstats-Accepted", "0")
			c.Header("X-Zenstats-Rejected", fmt.Sprint(len(events)))
			c.JSON(http.StatusBadRequest, "too many batch events")
			return
		}
		if isDatacenterOrThreatIP(c) {
			monitor.Outcome("ingress", "filtered", "datacenter_or_threat", len(events))
			c.Header("X-Zenstats-Accepted", "0")
			c.Header("X-Zenstats-Rejected", "0")
			c.Header("X-Zenstats-Filtered", fmt.Sprint(len(events)))
			c.JSON(http.StatusAccepted, "ok")
			return
		}
		requests := make([]*model.EventRequest, 0, len(events))
		rollbacks := make([]func(), 0, len(events))
		rejected, responseStatus := 0, http.StatusAccepted
		for _, temp := range events {
			req, rollback, reason, status := prepareEvent(c, siteService, temp)
			if req == nil {
				monitor.Outcome("ingress", "rejected", reason, 1)
				rejected++
				responseStatus = status
				continue
			}
			requests = append(requests, req)
			rollbacks = append(rollbacks, rollback)
		}
		if len(requests) > 0 {
			queue := globals.GetQueue()
			err := monitor.Accept(len(requests), func(records []*ingestion.Record) error {
				if queue == nil {
					return fmt.Errorf("queue unavailable")
				}
				for i := range requests {
					requests[i].Collection = records[i]
				}
				return queue.EnqueueMany(requests)
			})
			if err != nil {
				for _, rollback := range rollbacks {
					rollback()
				}
				monitor.Outcome("ingress", "rejected", "queue_unavailable", len(requests))
				c.Header("X-Zenstats-Accepted", "0")
				c.Header("X-Zenstats-Rejected", fmt.Sprint(rejected+len(requests)))
				c.Header("Retry-After", "5")
				c.JSON(http.StatusServiceUnavailable, "event queue unavailable")
				return
			}
			responseStatus = http.StatusAccepted
		}
		c.Header("X-Zenstats-Accepted", fmt.Sprint(len(requests)))
		c.Header("X-Zenstats-Rejected", fmt.Sprint(rejected))
		// Mixed batches preserve their admitted events; headers disclose rejected
		// children. Queue admission itself is atomic, so 503 never hides a prefix.
		if responseStatus == http.StatusServiceUnavailable {
			c.Header("Retry-After", "5")
		}
		if responseStatus == http.StatusAccepted {
			c.JSON(responseStatus, "ok")
		} else {
			c.JSON(responseStatus, "event rejected")
		}
	}
}

// prepareEvent applies the same admission rules to single and batched events.
func prepareEvent(c *gin.Context, siteService *service.SiteService, temp model.TempEventRequest) (*model.EventRequest, func(), string, int) {
	if temp.EventName == "" || temp.EventName == "batch" || temp.Domain == "" || temp.URL == "" {
		return nil, nil, "invalid_event", http.StatusBadRequest
	}
	site, err := siteService.GetSiteByDomain(c.Request.Context(), temp.Domain)
	if errors.Is(err, service.ErrSiteNotFound) {
		return nil, nil, "unknown_site", http.StatusBadRequest
	}
	if err != nil {
		return nil, nil, "site_lookup_failed", http.StatusServiceUnavailable
	}
	if !site.IsVerified {
		return nil, nil, "site_not_verified", http.StatusBadRequest
	}
	if !verifyRequestOrigin(c, temp.Domain, site.AllowedOrigins) {
		return nil, nil, "origin_not_allowed", http.StatusForbidden
	}
	if !limiter.allow(temp.Domain, site.IngestRateLimitScaleSeconds, site.IngestLimitPerMinute) {
		return nil, nil, "rate_limited", http.StatusTooManyRequests
	}
	rollback := func() {}
	ownerID, ownerErr := siteService.GetSiteOwnerUserID(c, site.ID)
	if ownerErr == nil && ownerID > 0 {
		user, err := service.GetUserService().GetUserWithConfig(c, ownerID)
		if err == nil && user.Edges.UserConfig != nil && user.Edges.UserConfig.Edges.Group != nil {
			var allowed bool
			rollback, allowed = event.GetMonthlyQuota().Reserve(ownerID, int64(user.Edges.UserConfig.Edges.Group.MaxMonthlyEvents))
			if !allowed {
				return nil, nil, "monthly_quota", http.StatusTooManyRequests
			}
		}
	}
	return newEventRequest(temp, temp.Domain, c), rollback, "", http.StatusAccepted
}
