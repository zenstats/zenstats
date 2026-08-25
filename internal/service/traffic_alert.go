package service

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/config"
	atypes "github.com/zenstats/zenstats/internal/api/types"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/site"
	"github.com/zenstats/zenstats/pkg/globals"
	"github.com/zenstats/zenstats/pkg/scheduler"
)

// TrafficAlertService detects traffic anomalies by comparing PV/UV against the same hour last week.
type TrafficAlertService struct {
	db          *ent.Client
	siteService *SiteService
	userService *UserService
	state       *alertState
}

// alertState is per-process de-duplication state for the hourly cron.
//
//   - lastDailyEval: siteID -> site-local date ("2006-01-02") for which the daily comparison has
//     already been evaluated. The cron fires hourly, but daily mode must run once per local day.
//   - activeHourly: siteID -> IsSpike of an anomaly that is currently ongoing. While an anomaly
//     persists across consecutive hours only the first one is emailed; the entry is cleared when a
//     check comes back normal (or re-alerted when the direction flips spike<->drop).
//
// Limitation: the site schema has no persisted last-alert column, so this state lives in memory.
// With multiple API replicas each replica de-duplicates independently (at most one alert per
// replica per day/anomaly window), and a restart resets the state.
type alertState struct {
	mu            sync.Mutex
	lastDailyEval map[int64]string
	activeHourly  map[int64]bool
}

func newAlertState() *alertState {
	return &alertState{
		lastDailyEval: make(map[int64]string),
		activeHourly:  make(map[int64]bool),
	}
}

// shouldEvalDaily returns true if the daily comparison has not yet been evaluated for localDate.
func (st *alertState) shouldEvalDaily(siteID int64, localDate string) bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.lastDailyEval[siteID] != localDate
}

// markDailyEvaluated records that the daily comparison for localDate has been evaluated.
func (st *alertState) markDailyEvaluated(siteID int64, localDate string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.lastDailyEval[siteID] = localDate
}

// shouldSendHourly reports whether an hourly anomaly should be emailed, and updates the
// active-anomaly window. anomaly == nil means the current hour is normal and clears the window.
func (st *alertState) shouldSendHourly(siteID int64, anomaly *alertData) bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	if anomaly == nil {
		delete(st.activeHourly, siteID)
		return false
	}
	if prev, active := st.activeHourly[siteID]; active && prev == anomaly.IsSpike {
		return false // same anomaly still ongoing: already alerted at its start
	}
	st.activeHourly[siteID] = anomaly.IsSpike
	return true
}

var getTrafficAlertService = sync.OnceValue(func() *TrafficAlertService {
	db := globals.GetDB()
	return &TrafficAlertService{
		db:          db.Client,
		siteService: GetSiteService(),
		userService: GetUserService(),
		state:       newAlertState(),
	}
})

func GetTrafficAlertService() *TrafficAlertService {
	return getTrafficAlertService()
}

// CheckAndAlert checks traffic changes for all verified sites and sends alerts when thresholds are exceeded.
// Uses per-site thresholds, defaults to 50%.
func (s *TrafficAlertService) CheckAndAlert(_ float64) {
	ctx := context.Background()
	slog.Info("running traffic anomaly check")

	if !smtpConfigured() {
		slog.Warn("smtp not configured, skipping traffic anomaly check")
		return
	}

	sites, err := s.db.Site.Query().
		Where(site.IsVerified(true)).
		Where(site.TrafficAlertEnabled(true)).
		All(ctx)
	if err != nil {
		slog.Error("failed to list verified sites for anomaly check", "error", err)
		return
	}

	for _, siteEnt := range sites {
		threshold := float64(siteEnt.TrafficAlertThreshold) / 100.0
		if threshold <= 0 {
			threshold = 0.5 // Default 50%
		}
		s.checkAndAlertSite(ctx, siteEnt, threshold)
	}

	slog.Info("traffic anomaly check completed", "sites", len(sites))
}

// checkAndAlertSite checks a single site for anomalies and sends alerts.
func (s *TrafficAlertService) checkAndAlertSite(ctx context.Context, siteEnt *ent.Site, threshold float64) {
	ownerID, err := s.siteService.GetSiteOwnerUserID(ctx, siteEnt.ID)
	if err != nil || ownerID == 0 {
		return
	}
	usr, err := s.userService.GetUserByID(ctx, ownerID)
	if err != nil || usr.Email == "" {
		return
	}

	now := time.Now()
	isDaily := siteEnt.TrafficAlertInterval == "daily"

	// Daily mode: evaluate once per site-local day (the cron itself fires hourly).
	var localDate string
	if isDaily {
		localDate = now.In(siteLocation(siteEnt.Timezone)).Format("2006-01-02")
		if !s.state.shouldEvalDaily(siteEnt.ID, localDate) {
			return
		}
	}

	alert, err := s.detectAnomalyAt(ctx, siteEnt, ownerID, threshold, now)
	if err != nil {
		// Transient failure: leave state untouched so the next hourly tick retries.
		slog.Warn("failed to check anomaly", "domain", siteEnt.Domain, "error", err)
		return
	}

	if isDaily {
		s.state.markDailyEvaluated(siteEnt.ID, localDate)
	} else if !s.state.shouldSendHourly(siteEnt.ID, alert) {
		if alert != nil {
			slog.Debug("traffic anomaly still ongoing, alert suppressed", "domain", siteEnt.Domain)
		}
		return
	}

	if alert != nil {
		// Site owner
		s.sendAlert(usr.Email, siteEnt.Domain, alert)
		// Additional recipients
		if siteEnt.TrafficAlertRecipients != nil && *siteEnt.TrafficAlertRecipients != "" {
			for _, r := range splitRecipients(*siteEnt.TrafficAlertRecipients) {
				if r != usr.Email {
					s.sendAlert(r, siteEnt.Domain, alert)
				}
			}
		}
	}
}

// alertData holds alert information.
type alertData struct {
	CurrentVisitors   float64
	PreviousVisitors  float64
	CurrentPageviews  float64
	PreviousPageviews float64
	VisitorsChange    float64 // Percentage, positive = spike
	PageviewsChange   float64
	IsSpike           bool // true=spike, false=drop
	Hour              string
	Threshold         float64 // Alert threshold percentage (e.g. 50)
}

// siteLocation resolves a site timezone name, falling back to UTC.
func siteLocation(timezone string) *time.Location {
	loc, err := time.LoadLocation(timezone)
	if err != nil || loc == nil {
		return time.UTC
	}
	return loc
}

// alertWindow holds the current/previous comparison ranges as site-local strings.
// The stats layer parses custom From/To in the site's timezone, so these MUST be
// formatted in site-local time, never server-local time.
type alertWindow struct {
	From, To         string
	PrevFrom, PrevTo string
	Label            string
}

// computeAlertWindow derives the comparison window for the given instant.
//
// daily:  yesterday (a complete site-local day) vs the same weekday one week before yesterday.
//
//	Comparing "today so far" against a full day would report huge fake drops in the
//	early hours of every day.
//
// hourly: the last completed site-local hour vs the same hour one week earlier.
func computeAlertWindow(now time.Time, timezone, interval string) alertWindow {
	loc := siteLocation(timezone)
	local := now.In(loc)

	if interval == "daily" {
		yesterday := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -1)
		weekBefore := yesterday.AddDate(0, 0, -7)
		y, w := yesterday.Format("2006-01-02"), weekBefore.Format("2006-01-02")
		return alertWindow{From: y, To: y, PrevFrom: w, PrevTo: w, Label: y + " vs " + w}
	}

	// Default to hourly: truncate to the local hour, then step back one hour.
	lastHour := time.Date(local.Year(), local.Month(), local.Day(), local.Hour(), 0, 0, 0, loc).Add(-time.Hour)
	lastHourEnd := lastHour.Add(time.Hour)
	sameHourLastWeek := lastHour.AddDate(0, 0, -7)
	const layout = "2006-01-02 15:04"
	return alertWindow{
		From:     lastHour.Format(layout),
		To:       lastHourEnd.Format(layout),
		PrevFrom: sameHourLastWeek.Format(layout),
		PrevTo:   sameHourLastWeek.Add(time.Hour).Format(layout),
		Label:    lastHour.Format("15:04") + " ~ " + lastHourEnd.Format("15:04") + " vs same period last week",
	}
}

// detectAnomaly checks a single site for traffic anomalies as of now.
func (s *TrafficAlertService) detectAnomaly(ctx context.Context, siteEnt *ent.Site, ownerID int64, threshold float64) (*alertData, error) {
	return s.detectAnomalyAt(ctx, siteEnt, ownerID, threshold, time.Now())
}

// detectAnomalyAt checks a single site for traffic anomalies relative to the given instant.
// Uses site interval config: hourly compares the last hour vs the same hour last week,
// daily compares yesterday vs the same weekday last week (see computeAlertWindow).
func (s *TrafficAlertService) detectAnomalyAt(ctx context.Context, siteEnt *ent.Site, ownerID int64, threshold float64, now time.Time) (*alertData, error) {
	statsSvc := GetStatsService()
	if statsSvc == nil {
		return nil, fmt.Errorf("stats service unavailable")
	}

	win := computeAlertWindow(now, siteEnt.Timezone, siteEnt.TrafficAlertInterval)
	fromStr, toStr, prevFromStr, prevToStr, timeLabel := win.From, win.To, win.PrevFrom, win.PrevTo, win.Label

	ginCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ginCtx.Set("user_id", ownerID)

	currentReq := &atypes.StatsRequest{
		Period: "custom",
		From:   fromStr,
		To:     toStr,
	}
	currentAgg, err := statsSvc.GetAggregate(ginCtx, siteEnt.ID, currentReq)
	if err != nil {
		return nil, fmt.Errorf("current aggregate: %w", err)
	}

	prevReq := &atypes.StatsRequest{
		Period: "custom",
		From:   prevFromStr,
		To:     prevToStr,
	}
	prevAgg, err := statsSvc.GetAggregate(ginCtx, siteEnt.ID, prevReq)
	if err != nil {
		return nil, fmt.Errorf("previous aggregate: %w", err)
	}

	currentVisitors := toFloat(currentAgg.Results["visitors"].Value)
	previousVisitors := toFloat(prevAgg.Results["visitors"].Value)
	currentPageviews := toFloat(currentAgg.Results["pageviews"].Value)
	previousPageviews := toFloat(prevAgg.Results["pageviews"].Value)

	visitorsChange := pctChange(currentVisitors, previousVisitors)
	pageviewsChange := pctChange(currentPageviews, previousPageviews)

	maxChange := math.Abs(visitorsChange)
	if math.Abs(pageviewsChange) > maxChange {
		maxChange = math.Abs(pageviewsChange)
	}
	if maxChange < threshold*100 {
		return nil, nil
	}

	return &alertData{
		CurrentVisitors:   currentVisitors,
		PreviousVisitors:  previousVisitors,
		CurrentPageviews:  currentPageviews,
		PreviousPageviews: previousPageviews,
		VisitorsChange:    visitorsChange,
		PageviewsChange:   pageviewsChange,
		IsSpike:           visitorsChange > 0,
		Hour:              timeLabel,
		Threshold:         threshold * 100,
	}, nil
}

func pctChange(current, previous float64) float64 {
	if previous == 0 {
		if current > 0 {
			return 100 // From 0 to traffic, treated as +100%
		}
		return 0
	}
	return ((current - previous) / previous) * 100
}

// sendAlert sends a traffic anomaly alert email through the shared SMTP sender in email.go.
// Skips (single Warn, no EmailLog row) when SMTP is not configured.
func (s *TrafficAlertService) sendAlert(email, domain string, alert *alertData) {
	if !smtpConfigured() {
		slog.Warn("smtp not configured, traffic alert not sent", "domain", domain)
		return
	}

	direction, icon := alertDirection(alert.IsSpike)
	subject := fmt.Sprintf("%s Zenstats traffic %s alert — %s", icon, direction, domain)
	body := renderAlertHTML(config.Conf.BaseURL, domain, alert)

	err := sendEmail(email, subject, body, EmailTypeTrafficAlert, nil)
	if err != nil {
		slog.Warn("failed to send traffic alert", "domain", domain, "error", err)
		slog.Debug("traffic alert recipient", "domain", domain, "email", email)
	} else {
		slog.Info("traffic alert sent",
			"domain", domain,
			"visitors_change", fmt.Sprintf("%.0f%%", alert.VisitorsChange),
			"pageviews_change", fmt.Sprintf("%.0f%%", alert.PageviewsChange),
		)
	}
}

func alertDirection(isSpike bool) (direction, icon string) {
	if isSpike {
		return "Spike", "📈"
	}
	return "Drop", "📉"
}

// renderAlertHTML renders the alert email body. domain is user-controlled (site domain) and is
// HTML-escaped wherever it is interpolated; the stats link is built with net/url path escaping.
func renderAlertHTML(baseURL, domain string, alert *alertData) string {
	direction, icon := alertDirection(alert.IsSpike)
	statsURL := strings.TrimRight(baseURL, "/") + "/sites/" + url.PathEscape(domain) + "/stats"

	visitorsSign := "+"
	if alert.VisitorsChange < 0 {
		visitorsSign = ""
	}
	pageviewsSign := "+"
	if alert.PageviewsChange < 0 {
		pageviewsSign = ""
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family:'Helvetica Neue',Helvetica,Arial,sans-serif; max-width:560px; margin:0 auto; padding:0; color:#1a1a2e;">
  <div style="background:%s; padding:32px 24px; text-align:center;">
    <div style="font-size:24px;">%s</div>
    <div style="font-size:20px; font-weight:700; color:#fff; margin-top:8px;">%s  Traffic %s</div>
    <div style="font-size:13px; color:rgba(255,255,255,0.75); margin-top:4px;">%s</div>
  </div>
  <div style="padding:24px;">
    <table width="100%%" cellpadding="0" cellspacing="0" style="border-collapse:collapse; margin-bottom:20px;">
      <tr>
        <td style="padding:12px; background:#f8f9fa; border-radius:8px; text-align:center; width:50%%;">
          <div style="font-size:11px; font-weight:700; color:#888;">Visitors</div>
          <div style="font-size:20px; font-weight:700;">%.0f</div>
          <div style="font-size:13px; color:%s;">%s%.0f%% vs last week</div>
        </td>
        <td style="width:8px;"></td>
        <td style="padding:12px; background:#f8f9fa; border-radius:8px; text-align:center; width:50%%;">
          <div style="font-size:11px; font-weight:700; color:#888;">Pageviews</div>
          <div style="font-size:20px; font-weight:700;">%.0f</div>
          <div style="font-size:13px; color:%s;">%s%.0f%% vs last week</div>
        </td>
      </tr>
    </table>
    <div style="border-top:1px solid #e5e7eb; padding-top:16px;">
      <p style="font-size:13px; color:#666;">上周同时段：Visitors %.0f / Pageviews %.0f</p>
      <p style="font-size:13px; color:#666;">本时段：Visitors %.0f / Pageviews %.0f</p>
    </div>
    <div style="margin-top:20px; text-align:center;">
      <a href="%s" style="display:inline-block; background:#4338ca; color:#fff; padding:10px 24px; border-radius:6px; text-decoration:none; font-size:14px;">View full stats</a>
    </div>
    <div style="border-top:1px solid #e5e7eb; margin-top:20px; padding-top:12px; text-align:center;">
      <p style="font-size:11px; color:#9ca3af;">This alert was automatically detected by Zenstats. Threshold: change exceeds %.0f%%。</p>
    </div>
  </div>
</body>
</html>`,
		// header color
		alertColor(alert.IsSpike),
		icon,
		esc(domain), direction,
		esc(alert.Hour),
		// visitors card
		alert.CurrentVisitors,
		alertColor(alert.IsSpike), visitorsSign, alert.VisitorsChange,
		// pageviews card
		alert.CurrentPageviews,
		alertColor(alert.IsSpike), pageviewsSign, alert.PageviewsChange,
		// detail
		alert.PreviousVisitors, alert.PreviousPageviews,
		alert.CurrentVisitors, alert.CurrentPageviews,
		// link
		esc(statsURL),
		// threshold
		alert.Threshold,
	)
}

func alertColor(isSpike bool) string {
	if isSpike {
		return "#e53e3e" // 红色：Spike
	}
	return "#3182ce" // 蓝色：Drop
}

// HourlyTrafficCheck 定时任务入口（每小时整点执行）。
// hourly 站点：对比上一完整小时与上周同小时，同一持续异常只在起始小时告警一次（见 alertState）。
// daily 站点：每个站点本地日只评估一次，对比昨天（完整日）与上周同一天。
func (s *TrafficAlertService) HourlyTrafficCheck(_ any) {
	s.CheckAndAlert(0) // 阈值从 DB 读取，参数忽略
}

// splitRecipients 分割逗号分隔的收件人列表。
func splitRecipients(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// RegisterTrafficAlertCron 注册流量异常检测定时任务（每小时整点 UTC）。
func RegisterTrafficAlertCron() {
	cron := scheduler.GetCronManager()
	svc := GetTrafficAlertService()
	cron.AddJob("0 0 * * * *", svc.HourlyTrafficCheck, nil)
	slog.Info("registered traffic anomaly alert cron job (hourly, UTC)")
}

// TestAlert 手动触发一次告警检测并返回结果（供 CLI 测试用）。
// 返回告警 HTML 内容和可能的错误。
func TestAlert(domain, email string) (html string, err error) {
	svc := GetTrafficAlertService()
	ctx := context.Background()

	siteEnt, err := svc.siteService.GetSiteByDomain(ctx, domain)
	if err != nil {
		return "", fmt.Errorf("site not found: %s", domain)
	}

	ownerID, err := svc.siteService.GetSiteOwnerUserID(ctx, siteEnt.ID)
	if err != nil || ownerID == 0 {
		return "", fmt.Errorf("site owner not found")
	}

	threshold := float64(siteEnt.TrafficAlertThreshold) / 100.0
	if threshold <= 0 {
		threshold = 0.5
	}

	alert, err := svc.detectAnomaly(ctx, siteEnt, ownerID, threshold)
	if err != nil {
		return "", fmt.Errorf("anomaly detection failed: %w", err)
	}
	if alert == nil {
		return "", fmt.Errorf("no anomaly detected (change below threshold %.0f%%)", threshold*100)
	}

	// 如果有指定邮箱，发送告警
	if email != "" {
		svc.sendAlert(email, siteEnt.Domain, alert)
		// Additional recipients
		if siteEnt.TrafficAlertRecipients != nil && *siteEnt.TrafficAlertRecipients != "" {
			for _, r := range splitRecipients(*siteEnt.TrafficAlertRecipients) {
				if r != email {
					svc.sendAlert(r, siteEnt.Domain, alert)
				}
			}
		}
	}

	return fmt.Sprintf(`Traffic Alert: %s
Visitors: %.0f → %.0f (%.0f%%)
Pageviews: %.0f → %.0f (%.0f%%)
Period: %s
Threshold: %.0f%%`,
		domain,
		alert.PreviousVisitors, alert.CurrentVisitors, alert.VisitorsChange,
		alert.PreviousPageviews, alert.CurrentPageviews, alert.PageviewsChange,
		alert.Hour,
		threshold*100,
	), nil
}

// TrafficAlertTestResult 管理员测试接口返回结果。
type TrafficAlertTestResult struct {
	Domain            string  `json:"domain"`
	Interval          string  `json:"interval"`
	Period            string  `json:"period"`
	Threshold         float64 `json:"threshold"`
	AnomalyDetected   bool    `json:"anomaly_detected"`
	IsSpike           bool    `json:"is_spike,omitempty"`
	CurrentVisitors   float64 `json:"current_visitors"`
	PreviousVisitors  float64 `json:"previous_visitors"`
	VisitorsChange    float64 `json:"visitors_change"`
	CurrentPageviews  float64 `json:"current_pageviews"`
	PreviousPageviews float64 `json:"previous_pageviews"`
	PageviewsChange   float64 `json:"pageviews_change"`
	EmailSent         bool    `json:"email_sent"`
	EmailRecipient    string  `json:"email_recipient,omitempty"`
}

// RunTestForSite 对指定站点执行一次检测，不受 enabled 开关限制。
// 若 email 非空则发送测试邮件。供管理员 API 调用。
func (s *TrafficAlertService) RunTestForSite(ctx context.Context, siteID int64, email string) (*TrafficAlertTestResult, error) {
	siteEnt, err := s.db.Site.Query().
		Where(site.ID(siteID)).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("site not found: %d", siteID)
	}

	ownerID, err := s.siteService.GetSiteOwnerUserID(ctx, siteEnt.ID)
	if err != nil || ownerID == 0 {
		return nil, fmt.Errorf("site owner not found")
	}

	threshold := float64(siteEnt.TrafficAlertThreshold) / 100.0
	if threshold <= 0 {
		threshold = 0.5
	}

	alert, err := s.detectAnomaly(ctx, siteEnt, ownerID, threshold)
	if err != nil {
		return nil, fmt.Errorf("anomaly detection failed: %w", err)
	}

	result := &TrafficAlertTestResult{
		Domain:    siteEnt.Domain,
		Interval:  siteEnt.TrafficAlertInterval,
		Threshold: threshold * 100,
	}

	if alert == nil {
		return result, nil
	}

	result.AnomalyDetected = true
	result.IsSpike = alert.IsSpike
	result.Period = alert.Hour
	result.CurrentVisitors = alert.CurrentVisitors
	result.PreviousVisitors = alert.PreviousVisitors
	result.VisitorsChange = alert.VisitorsChange
	result.CurrentPageviews = alert.CurrentPageviews
	result.PreviousPageviews = alert.PreviousPageviews
	result.PageviewsChange = alert.PageviewsChange

	if email != "" {
		s.sendAlert(email, siteEnt.Domain, alert)
		result.EmailSent = true
		result.EmailRecipient = email
	}

	return result, nil
}
