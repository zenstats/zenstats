package service

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net/http/httptest"
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
	"gopkg.in/gomail.v2"
)

// TrafficAlertService 流量异常检测服务，每小时对比上周同小时 PV/UV 变化，超过阈值发送告警。
type TrafficAlertService struct {
	db          *ent.Client
	siteService *SiteService
	userService *UserService
}

var getTrafficAlertService = sync.OnceValue(func() *TrafficAlertService {
	db := globals.GetDB()
	return &TrafficAlertService{
		db:          db.Client,
		siteService: GetSiteService(),
		userService: GetUserService(),
	}
})

func GetTrafficAlertService() *TrafficAlertService {
	return getTrafficAlertService()
}

// CheckAndAlert 检查所有已验证站点的流量变化，超过阈值时发送告警。
// 优先使用站点自定义阈值，未设置则默认 50%。
func (s *TrafficAlertService) CheckAndAlert(_ float64) {
	ctx := context.Background()
	slog.Info("running traffic anomaly check")

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
			threshold = 0.5 // 默认 50%
		}
		s.checkAndAlertSite(ctx, siteEnt, threshold)
	}

	slog.Info("traffic anomaly check completed", "sites", len(sites))
}

// checkAndAlertSite 检查单个站点的流量异常并发送告警。
func (s *TrafficAlertService) checkAndAlertSite(ctx context.Context, siteEnt *ent.Site, threshold float64) {
	ownerID, err := s.siteService.GetSiteOwnerUserID(ctx, siteEnt.ID)
	if err != nil || ownerID == 0 {
		return
	}
	usr, err := s.userService.GetUserByID(ctx, ownerID)
	if err != nil || usr.Email == "" {
		return
	}

	alert, err := s.detectAnomaly(ctx, siteEnt, ownerID, threshold)
	if err != nil {
		slog.Warn("failed to check anomaly", "domain", siteEnt.Domain, "error", err)
		return
	}
	if alert != nil {
		// 站点所有者
		s.sendAlert(usr.Email, siteEnt.Domain, alert)
		// 额外收件人
		if siteEnt.TrafficAlertRecipients != nil && *siteEnt.TrafficAlertRecipients != "" {
			for _, r := range splitRecipients(*siteEnt.TrafficAlertRecipients) {
				if r != usr.Email {
					s.sendAlert(r, siteEnt.Domain, alert)
				}
			}
		}
	}
}

// alertData 告警数据。
type alertData struct {
	CurrentVisitors  float64
	PreviousVisitors float64
	CurrentPageviews float64
	PreviousPageviews float64
	VisitorsChange   float64 // 百分比，正=上升
	PageviewsChange  float64
	IsSpike          bool   // true=激增, false=骤降
	Hour             string
}

// detectAnomaly 检测单个站点的流量异常。
// 根据站点配置的 interval：hourly 对比上一小时 vs 上周同小时，daily 对比今天 vs 上周同日。
func (s *TrafficAlertService) detectAnomaly(ctx context.Context, siteEnt *ent.Site, ownerID int64, threshold float64) (*alertData, error) {
	statsSvc := GetStatsService()
	if statsSvc == nil {
		return nil, fmt.Errorf("stats service unavailable")
	}

	now := time.Now()
	var fromStr, toStr, prevFromStr, prevToStr string
	var timeLabel string

	if siteEnt.TrafficAlertInterval == "daily" {
		today := now.Format("2006-01-02")
		weekAgo := now.AddDate(0, 0, -7).Format("2006-01-02")
		fromStr, toStr = today, today
		prevFromStr, prevToStr = weekAgo, weekAgo
		timeLabel = today + " vs " + weekAgo
	} else {
		// 默认 hourly
		lastHour := now.Truncate(time.Hour).Add(-time.Hour)
		sameHourLastWeek := lastHour.AddDate(0, 0, -7)
		fromStr = lastHour.Format("2006-01-02 15:04")
		toStr = lastHour.Add(time.Hour).Format("2006-01-02 15:04")
		prevFromStr = sameHourLastWeek.Format("2006-01-02 15:04")
		prevToStr = sameHourLastWeek.Add(time.Hour).Format("2006-01-02 15:04")
		timeLabel = lastHour.Format("15:04") + " ~ " + lastHour.Add(time.Hour).Format("15:04") + " vs 上周同时段"
	}

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
	}, nil
}

func pctChange(current, previous float64) float64 {
	if previous == 0 {
		if current > 0 {
			return 100 // 从 0 到有流量，视为 +100%
		}
		return 0
	}
	return ((current - previous) / previous) * 100
}

// sendAlert 发送流量异常告警邮件。
func (s *TrafficAlertService) sendAlert(email, domain string, alert *alertData) {
	cfg := config.Conf
	host := cfg.SMTP.Host
	port := cfg.SMTP.Port
	username := cfg.SMTP.Username
	password := cfg.SMTP.Password
	from := cfg.SMTP.From

	if host == "" {
		host = "localhost"
	}
	if port == 0 {
		port = 587
	}
	if from == "" {
		from = "noreply@zenstats.io"
	}

	direction := "骤降"
	icon := "📉"
	if alert.IsSpike {
		direction = "激增"
		icon = "📈"
	}

	subject := fmt.Sprintf("%s Zenstats 流量%s告警 — %s", icon, direction, domain)

	visitorsSign := "+"
	if alert.VisitorsChange < 0 {
		visitorsSign = ""
	}
	pageviewsSign := "+"
	if alert.PageviewsChange < 0 {
		pageviewsSign = ""
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family:'Helvetica Neue',Helvetica,Arial,sans-serif; max-width:560px; margin:0 auto; padding:0; color:#1a1a2e;">
  <div style="background:%s; padding:32px 24px; text-align:center;">
    <div style="font-size:24px;">%s</div>
    <div style="font-size:20px; font-weight:700; color:#fff; margin-top:8px;">%s 流量%s</div>
    <div style="font-size:13px; color:rgba(255,255,255,0.75); margin-top:4px;">%s</div>
  </div>
  <div style="padding:24px;">
    <table width="100%%" cellpadding="0" cellspacing="0" style="border-collapse:collapse; margin-bottom:20px;">
      <tr>
        <td style="padding:12px; background:#f8f9fa; border-radius:8px; text-align:center; width:50%%;">
          <div style="font-size:11px; font-weight:700; color:#888;">访客</div>
          <div style="font-size:20px; font-weight:700;">%.0f</div>
          <div style="font-size:13px; color:%s;">%s%.0f%% vs 上周</div>
        </td>
        <td style="width:8px;"></td>
        <td style="padding:12px; background:#f8f9fa; border-radius:8px; text-align:center; width:50%%;">
          <div style="font-size:11px; font-weight:700; color:#888;">页面浏览</div>
          <div style="font-size:20px; font-weight:700;">%.0f</div>
          <div style="font-size:13px; color:%s;">%s%.0f%% vs 上周</div>
        </td>
      </tr>
    </table>
    <div style="border-top:1px solid #e5e7eb; padding-top:16px;">
      <p style="font-size:13px; color:#666;">上周同时段：访客 %.0f / 页面浏览 %.0f</p>
      <p style="font-size:13px; color:#666;">本时段：访客 %.0f / 页面浏览 %.0f</p>
    </div>
    <div style="margin-top:20px; text-align:center;">
      <a href="%s" style="display:inline-block; background:#4338ca; color:#fff; padding:10px 24px; border-radius:6px; text-decoration:none; font-size:14px;">查看完整统计</a>
    </div>
    <div style="border-top:1px solid #e5e7eb; margin-top:20px; padding-top:12px; text-align:center;">
      <p style="font-size:11px; color:#9ca3af;">本告警由 Zenstats 自动检测发送。阈值：变化超过 %.0f%%。</p>
    </div>
  </div>
</body>
</html>`,
		// header color
		alertColor(alert.IsSpike),
		icon,
		domain, direction,
		alert.Hour,
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
		cfg.BaseURL+"/"+domain,
		// threshold
		thresholdValue(),
	)

	m := gomail.NewMessage()
	m.SetHeader("From", from)
	m.SetHeader("To", email)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", html)

	d := gomail.NewDialer(host, port, username, password)
	if err := d.DialAndSend(m); err != nil {
		slog.Warn("failed to send traffic alert", "email", email, "error", err)
	} else {
		slog.Info("traffic alert sent",
			"domain", domain,
			"visitors_change", fmt.Sprintf("%.0f%%", alert.VisitorsChange),
			"pageviews_change", fmt.Sprintf("%.0f%%", alert.PageviewsChange),
		)
	}
}

func alertColor(isSpike bool) string {
	if isSpike {
		return "#e53e3e" // 红色：激增
	}
	return "#3182ce" // 蓝色：骤降
}

func thresholdValue() float64 {
	return 50 // 默认 50% 阈值，与 HourlyTrafficCheck 一致
}

// HourlyTrafficCheck 定时任务入口（每小时整点执行）。
// 对比上一完整小时与上周同小时的 PV/UV，每次检查不同时段，无重复告警。
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
	cron.AddJob("0 * * * *", svc.HourlyTrafficCheck, nil)
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
		// 额外收件人
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
