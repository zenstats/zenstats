// Package external 处理外部事件采集请求，接收前端 SDK 上报的埋点事件数据。
package external

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/event"
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

// verifyRequestOrigin 验证请求来源是否匹配域名。
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

// setEventRequestDefaults 设置 EventRequest 的默认值
func setEventRequestDefaults(req *model.EventRequest, c *gin.Context) {
	req.Timestamp = time.Now()
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
//	@Router			/event [post]
func Event(siteService *service.SiteService) gin.HandlerFunc {
	return func(c *gin.Context) {
		slog.Debug("received event")

		var tempReq model.TempEventRequest
		body, err := c.GetRawData()
		if err != nil {
			slog.Debug("failed to get raw data", "error", err)
			c.JSON(http.StatusBadRequest, "bad")
			return
		}
		slog.Debug("request body", "body", string(body))

		err = json.Unmarshal(body, &tempReq)
		if err != nil {
			slog.Debug("failed to unmarshal request body", "error", err)
			c.JSON(http.StatusBadRequest, "bad")
			return
		}

		// Set default value for Interactive if not provided
		if tempReq.Interactive == nil {
			trueVal := json.RawMessage("true")
			tempReq.Interactive = &trueVal
		}

		// 处理前端 SDK 批量发送的 batch 事件（n: 'batch', e: [...]）
		if tempReq.EventName == "batch" && len(tempReq.Events) > 0 {
			// 丢弃来自数据中心/威胁 IP 的 batch 请求
			if isDatacenterOrThreatIP(c) {
				slog.Debug("dropping datacenter/threat IP batch event", "ip", iputil.ClientIP(c.Request))
				c.JSON(http.StatusAccepted, "ok")
				return
			}
			for _, subEvent := range tempReq.Events {
				// 子事件继承 batch 级别的 domain（如果未单独指定）
				subDomain := subEvent.Domain
				if subDomain == "" {
					subDomain = tempReq.Domain
				}

				can, err := siteService.IsDomainInList(c, subDomain)
				if err != nil || !can {
					continue
				}

				site, err := siteService.GetVerifiedSiteByDomain(c, subDomain)
				if err != nil {
					continue
				}

				if !verifyRequestOrigin(c, subDomain, site.AllowedOrigins) {
					continue
				}

				// 限频检查
				if !limiter.allow(subDomain, site.IngestRateLimitScaleSeconds, site.IngestLimitPerMinute) {
					continue
				}

				// 月事件数配额检查
				ownerUserID, ownerErr := siteService.GetSiteOwnerUserID(c, site.ID)
				if ownerErr == nil && ownerUserID > 0 {
					userService := service.GetUserService()
					if user, userErr := userService.GetUserWithConfig(c, ownerUserID); userErr == nil &&
						user.Edges.UserConfig != nil && user.Edges.UserConfig.Edges.Group != nil {
						maxEvents := user.Edges.UserConfig.Edges.Group.MaxMonthlyEvents
						if maxEvents != -1 {
							quota := event.GetMonthlyQuota()
							if quota.Get(ownerUserID) >= int64(maxEvents) {
								continue
							}
							quota.Increment(ownerUserID)
						} else {
							event.GetMonthlyQuota().Increment(ownerUserID)
						}
					}
				}

				// 子事件长度校验
				if len(subEvent.EventName) > 120 {
					subEvent.EventName = subEvent.EventName[:120]
				}
				if len(subEvent.URL) > 2000 {
					subEvent.URL = subEvent.URL[:2000]
				}

				req := model.EventRequest{
					Timestamp:      subEvent.Timestamp,
					Hash:           subEvent.Hash,
					EventName:      subEvent.EventName,
					JSVersion:      subEvent.JSVersion,
					URL:            subEvent.URL,
					Domain:         subDomain,
					Referrer:       subEvent.Referrer,
					Props:          subEvent.Props,
					EngagementTime: subEvent.EngagementTime,
					ScrollDepth:    subEvent.ScrollDepth,
				}

				setEventRequestDefaults(&req, c)
				parseInteractive(&req, &subEvent)

				queue := globals.GetQueue()
				_ = queue.Enqueue(&req)
			}
			c.JSON(http.StatusAccepted, "ok")
			return
		}

		// 丢弃来自数据中心/威胁 IP 的请求
		if isDatacenterOrThreatIP(c) {
			slog.Debug("dropping datacenter/threat IP event", "ip", iputil.ClientIP(c.Request))
			c.JSON(http.StatusAccepted, "ok")
			return
		}

		can, err := siteService.IsDomainInList(c, tempReq.Domain)
		if err != nil {
			slog.Debug("failed to check domain", "error", err)
			c.JSON(http.StatusBadRequest, "bad")
			return
		}
		if !can {
			slog.Debug("domain not allowed", "domain", tempReq.Domain)
			c.JSON(http.StatusBadRequest, "bad")
			return
		}

		// 验证检查：仅接受已验证站点的事件
		site, err := siteService.GetVerifiedSiteByDomain(c, tempReq.Domain)
		if err != nil {
			slog.Debug("site not verified", "domain", tempReq.Domain, "error", err)
			c.JSON(http.StatusBadRequest, "bad")
			return
		}

		// 验证请求来源
		if !verifyRequestOrigin(c, tempReq.Domain, site.AllowedOrigins) {
			slog.Warn("event rejected: invalid request origin",
				"domain", tempReq.Domain,
				"origin", c.GetHeader("Origin"),
				"referer", c.GetHeader("Referer"),
			)
			c.JSON(http.StatusForbidden, "forbidden")
			return
		}

		// 限频检查：仅在站点配置了限频时生效
		if site != nil {
			if !limiter.allow(tempReq.Domain, site.IngestRateLimitScaleSeconds, site.IngestLimitPerMinute) {
				slog.Warn("rate limit exceeded",
					"domain", tempReq.Domain,
					"rate_limit_scale_seconds", site.IngestRateLimitScaleSeconds,
					"limit_per_minute", site.IngestLimitPerMinute,
				)
				c.JSON(http.StatusTooManyRequests, "rate limit exceeded")
				return
			}
		}

		// 月事件数配额检查
		if site != nil {
			ownerUserID, err := siteService.GetSiteOwnerUserID(c, site.ID)
			if err == nil && ownerUserID > 0 {
				userService := service.GetUserService()
				user, err := userService.GetUserWithConfig(c, ownerUserID)
				if err == nil && user.Edges.UserConfig != nil && user.Edges.UserConfig.Edges.Group != nil {
					maxEvents := user.Edges.UserConfig.Edges.Group.MaxMonthlyEvents
					if maxEvents != -1 {
						quota := event.GetMonthlyQuota()
						// Check before incrementing so rejected events don't inflate the count.
						if quota.Get(ownerUserID) >= int64(maxEvents) {
							c.JSON(http.StatusTooManyRequests, "monthly event limit exceeded")
							return
						}
						quota.Increment(ownerUserID)
					} else {
						// 无限制但仍需计数
						event.GetMonthlyQuota().Increment(ownerUserID)
					}
				}
			}
		}

		// 长度校验
		const maxEventNameLength = 120
		const maxURLLength = 2000

		if len(tempReq.EventName) > maxEventNameLength {
			slog.Debug("event name too long, truncating", "name", tempReq.EventName)
			tempReq.EventName = tempReq.EventName[:maxEventNameLength]
		}
		if len(tempReq.URL) > maxURLLength {
			slog.Debug("url too long, truncating", "url", tempReq.URL)
			tempReq.URL = tempReq.URL[:maxURLLength]
		}

		req := model.EventRequest{
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
