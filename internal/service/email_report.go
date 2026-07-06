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
	"github.com/zenstats/zenstats/internal/service/stats"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/predicate"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/site"
	"github.com/zenstats/zenstats/pkg/globals"
	"github.com/zenstats/zenstats/pkg/scheduler"
	"gopkg.in/gomail.v2"
)

// EmailReportService sends periodic weekly/monthly email reports.
type EmailReportService struct {
	db          *ent.Client
	siteService *SiteService
	userService *UserService
}

var getEmailReportService = sync.OnceValue(func() *EmailReportService {
	db := globals.GetDB()
	return &EmailReportService{
		db:          db.Client,
		siteService: GetSiteService(),
		userService: GetUserService(),
	}
})

func GetEmailReportService() *EmailReportService {
	return getEmailReportService()
}

// sendMail sends an HTML email.
func (s *EmailReportService) sendMail(to, subject, html string) error {
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

	m := gomail.NewMessage()
	m.SetHeader("From", from)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", html)

	d := gomail.NewDialer(host, port, username, password)
	err := d.DialAndSend(m)
	GetEmailLogService().RecordSend(context.Background(), to, subject, EmailTypeReport, nil, err)
	return err
}

// SendWeeklyReports sends weekly reports for verified sites with weekly reports enabled.
func (s *EmailReportService) SendWeeklyReports(_ any) {
	ctx := context.Background()
	slog.Info("sending weekly email reports")
	s.processReports(ctx, "weekly", site.EmailReportWeekly(true))
}

// SendMonthlyReports sends monthly reports for verified sites with monthly reports enabled.
func (s *EmailReportService) SendMonthlyReports(_ any) {
	ctx := context.Background()
	slog.Info("sending monthly email reports")
	s.processReports(ctx, "monthly", site.EmailReportMonthly(true))
}

// processReports iterates verified sites with the given flag enabled and sends reports to their owners.
func (s *EmailReportService) processReports(ctx context.Context, interval string, filter predicate.Site) {
	sites, err := s.db.Site.Query().
		Where(site.IsVerified(true)).
		Where(filter).
		All(ctx)
	if err != nil {
		slog.Error("failed to list verified sites for reports", "error", err)
		return
	}

	for _, siteEnt := range sites {
		ownerID, err := s.siteService.GetSiteOwnerUserID(ctx, siteEnt.ID)
		if err != nil || ownerID == 0 {
			continue
		}

		usr, err := s.userService.GetUserByID(ctx, ownerID)
		if err != nil || usr.Email == "" {
			continue
		}

		html, err := s.buildReportHTML(ctx, siteEnt, ownerID, interval)
		if err != nil {
			slog.Warn("failed to build report", "domain", siteEnt.Domain, "error", err)
			continue
		}

		name := reportName(interval)
		subject := fmt.Sprintf("%s report for %s", name, siteEnt.Domain)
		if err := s.sendMail(usr.Email, subject, html); err != nil {
			slog.Warn("failed to send report email", "email", usr.Email, "error", err)
		}
	}

	slog.Info("email reports sent", "interval", interval, "sites", len(sites))
}

// ============================================================================
// Report Content Builder
// ============================================================================

// reportStats holds the statistics for a report.
type reportStats struct {
	Visitors      metricWithChange
	Pageviews     metricWithChange
	BounceRate    metricWithChange
	VisitDuration metricWithChange
	TopPages      []rankedItem
	TopSources    []rankedItem
}

type metricWithChange struct {
	Value  float64
	Change *float64 // Percentage change, nil means no comparison data
}

type rankedItem struct {
	Name     string
	Visitors float64
}

// buildReportHTML builds the report HTML.
func (s *EmailReportService) buildReportHTML(ctx context.Context, siteEnt *ent.Site, ownerID int64, interval string) (string, error) {
	from, to := calcDateRange(siteEnt.Timezone, interval)

	stats, err := s.queryStats(ctx, siteEnt, ownerID, from, to)
	if err != nil {
		slog.Warn("failed to query report stats", "domain", siteEnt.Domain, "error", err)
	}

	name := reportName(interval)
	displayName := siteEnt.Domain
	if siteEnt.Remark != "" {
		displayName = siteEnt.Remark
	}

	// Format date label in site local time
	loc, _ := time.LoadLocation(siteEnt.Timezone)
	if loc == nil {
		loc = time.UTC
	}
	toLocal := to.In(loc)

	statsError := ""
	if err != nil {
		statsError = err.Error()
	}

	return renderReportHTML(reportViewData{
		DisplayName:     displayName,
		ReportName:      name,
		DateLabel:       toLocal.Format("Jan 2, 2006"),
		From:            from.Format("Jan 2"),
		To:              toLocal.Format("Jan 2, 2006"),
		Stats:           stats,
		HasStats:        stats != nil,
		StatsError:      statsError,
		UnsubscribeNote: "To unsubscribe, disable email reports in site settings.",
	}), nil
}

type reportViewData struct {
	DisplayName     string
	ReportName      string
	DateLabel       string
	From            string
	To              string
	Stats           *reportStats
	HasStats        bool
	StatsError      string
	UnsubscribeNote string
}

// queryStats queries all statistics needed for the report.
func (s *EmailReportService) queryStats(ctx context.Context, siteEnt *ent.Site, ownerID int64, from, to time.Time) (stats *reportStats, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("stats service panic: %v", r)
			stats = nil
		}
	}()

	statsSvc := GetStatsService()
	if statsSvc == nil {
		return nil, fmt.Errorf("stats service unavailable")
	}

	slog.Debug("building report stats",
		"domain", siteEnt.Domain,
		"timezone", siteEnt.Timezone,
		"from", from.Format("2006-01-02"),
		"to", to.Format("2006-01-02"),
	)

	ginCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ginCtx.Set("user_id", ownerID)

	req := &atypes.StatsRequest{
		Period: "custom",
		From:   from.Format("2006-01-02"),
		To:     to.Format("2006-01-02"),
	}

	result := &reportStats{}

	// Current period aggregate
	agg, err := statsSvc.GetAggregate(ginCtx, siteEnt.ID, req)
	if err != nil {
		return nil, fmt.Errorf("aggregate query: %w", err)
	}
	if v, ok := agg.Results["visitors"]; ok {
		result.Visitors = metricWithChange{Value: toFloat(v.Value)}
	}
	if v, ok := agg.Results["pageviews"]; ok {
		result.Pageviews = metricWithChange{Value: toFloat(v.Value)}
	}
	if v, ok := agg.Results["bounce_rate"]; ok {
		result.BounceRate = metricWithChange{Value: toFloat(v.Value)}
	}
	if v, ok := agg.Results["visit_duration"]; ok {
		result.VisitDuration = metricWithChange{Value: toFloat(v.Value)}
	}

	// Previous period aggregate (manual comparison)
	days := int(to.Sub(from).Hours()/24) + 1
	prevFrom := from.AddDate(0, 0, -days)
	prevTo := to.AddDate(0, 0, -days)
	prevReq := &atypes.StatsRequest{
		Period: "custom",
		From:   prevFrom.Format("2006-01-02"),
		To:     prevTo.Format("2006-01-02"),
	}
	if prevAgg, err := statsSvc.GetAggregate(ginCtx, siteEnt.ID, prevReq); err == nil {
		if v, ok := prevAgg.Results["visitors"]; ok {
			result.Visitors.Change = calcChange(result.Visitors.Value, toFloat(v.Value))
		}
		if v, ok := prevAgg.Results["pageviews"]; ok {
			result.Pageviews.Change = calcChange(result.Pageviews.Value, toFloat(v.Value))
		}
		if v, ok := prevAgg.Results["bounce_rate"]; ok {
			result.BounceRate.Change = calcChange(result.BounceRate.Value, toFloat(v.Value))
		}
		if v, ok := prevAgg.Results["visit_duration"]; ok {
			result.VisitDuration.Change = calcChange(result.VisitDuration.Value, toFloat(v.Value))
		}
	}

	// Top 5 pages
	if pages, err := statsSvc.GetBreakdown(ginCtx, siteEnt.ID, req, "event:page", "visitors"); err == nil {
		for i, row := range pages.Data {
			if i >= 5 {
				break
			}
			result.TopPages = append(result.TopPages, rankedItem{
				Name:     fmt.Sprintf("%v", row["name"]),
				Visitors: toFloat(row["visitors"]),
			})
		}
	}

	// Top 5 sources (excluding Direct / None)
	if sources, err := statsSvc.GetBreakdown(ginCtx, siteEnt.ID, req, "visit:source", "visitors"); err == nil {
		for _, row := range sources.Data {
			if len(result.TopSources) >= 5 {
				break
			}
			name := fmt.Sprintf("%v", row["name"])
			if name == "Direct / None" || name == "" {
				continue
			}
			result.TopSources = append(result.TopSources, rankedItem{
				Name:     name,
				Visitors: toFloat(row["visitors"]),
			})
		}
	}

	return result, nil
}

// calcChange computes percentage change. Returns nil when prev=0.
func calcChange(current, prev float64) *float64 {
	if prev == 0 {
		return nil
	}
	ch := ((current - prev) / prev) * 100
	return &ch
}

// ============================================================================
// Helpers
// ============================================================================

// calcDateRange computes the date range based on site timezone and report interval.
// w weekly: last Monday ~ last Sunday
// m monthly: 1st ~ last day of previous month
func calcDateRange(timezone, interval string) (from, to time.Time) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}

	now := time.Now().In(loc)

	switch interval {
	case "weekly":
		weekday := now.Weekday()
		daysSinceMonday := int(weekday) - 1
		if daysSinceMonday < 0 {
			daysSinceMonday += 7
		}
		lastMonday := now.AddDate(0, 0, -(7 + daysSinceMonday))
		lastSunday := lastMonday.AddDate(0, 0, 6)
		from = time.Date(lastMonday.Year(), lastMonday.Month(), lastMonday.Day(), 0, 0, 0, 0, loc)
		to = time.Date(lastSunday.Year(), lastSunday.Month(), lastSunday.Day(), 23, 59, 59, 0, loc)

	case "monthly":
		firstOfThisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
		lastDayOfLastMonth := firstOfThisMonth.AddDate(0, 0, -1)
		from = time.Date(lastDayOfLastMonth.Year(), lastDayOfLastMonth.Month(), 1, 0, 0, 0, 0, loc)
		to = time.Date(lastDayOfLastMonth.Year(), lastDayOfLastMonth.Month(), lastDayOfLastMonth.Day(), 23, 59, 59, 0, loc)

	default:
		from = now.AddDate(0, 0, -7)
		to = now
	}

	// Return site-local time; getDateRange handles timezone conversion
	return
}

func reportName(interval string) string {
	switch interval {
	case "weekly":
		return "Weekly"
	case "monthly":
		// Return last month English name, e.g. "June"
		now := time.Now()
		lastMonth := now.AddDate(0, -1, 0)
		return lastMonth.Format("January")
	default:
		return "Stats"
	}
}

func toMetricWithChange(v any) metricWithChange {
	// Handle stats.MetricResult struct
	if mr, ok := v.(stats.MetricResult); ok {
		mc := metricWithChange{Value: toFloat(mr.Value)}
		if mr.Change != nil {
			mc.Change = mr.Change
		}
		return mc
	}

	switch val := v.(type) {
	case float64:
		return metricWithChange{Value: val}
	case int64:
		return metricWithChange{Value: float64(val)}
	case map[string]any:
		value, _ := toFloatSafe(val["value"])
		var change *float64
		if c, ok := toFloatSafe(val["change"]); ok {
			change = &c
		} else if cv, ok := toFloatSafe(val["comparison_value"]); ok && value > 0 {
			ch := ((value - cv) / cv) * 100
			change = &ch
		}
		return metricWithChange{Value: value, Change: change}
	default:
		return metricWithChange{Value: toFloat(v)}
	}
}

func toFloat(v any) float64 {
	f, _ := toFloatSafe(v)
	return f
}

func toFloatSafe(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case int64:
		return float64(val), true
	case uint64:
		return float64(val), true
	case int:
		return float64(val), true
	case *float64:
		if val != nil {
			return *val, true
		}
	case nil:
		return 0, false
	}
	return 0, false
}

// ============================================================================
// HTML Rendering
// ============================================================================

func formatLargeNumber(v float64) string {
	if v >= 1_000_000 {
		return fmt.Sprintf("%.1fM", v/1_000_000)
	}
	if v >= 1_000 {
		return fmt.Sprintf("%.1fk", v/1_000)
	}
	return fmt.Sprintf("%.0f", v)
}

func formatChange(v float64) string {
	if v == 0 {
		return "0%"
	}
	if v > 0 {
		return fmt.Sprintf("+%.0f%%", v)
	}
	return fmt.Sprintf("%.0f%%", v)
}

func changeColorClass(v float64, positiveIsGood bool) string {
	if v == 0 {
		return "#888"
	}
	good := v > 0
	if !positiveIsGood {
		good = !good
	}
	if good {
		return "#15803d"
	}
	return "#b91c1c"
}

// renderReportHTML renders the final HTML report.
func renderReportHTML(d reportViewData) string {
	var statsHTML string
	if d.StatsError != "" {
		statsHTML = fmt.Sprintf(`<p style="color:#b91c1c; text-align:center; padding:24px; background:#fef2f2; border-radius:8px; margin:16px 0;">
⚠️ Stats data load failed：%s<br>
<small>The report framework is working. Contact the admin to check ClickHouse connection and site data.</small></p>`, d.StatsError)
	} else if d.HasStats && d.Stats != nil {
		statsHTML = renderStatsSection(d.Stats)
	} else {
		statsHTML = `<p style="color:#999; text-align:center; padding:24px;">No statistics data for this period.</p>`
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"></head>
<body style="font-family:'Helvetica Neue',Helvetica,Arial,sans-serif; max-width:560px; margin:0 auto; padding:0; color:#1a1a2e;">
  <!-- Header -->
  <div style="background:#4338ca; padding:32px 24px; text-align:center;">
    <div style="font-size:20px; font-weight:700; color:#fff;">%s</div>
    <div style="font-size:13px; color:rgba(255,255,255,0.75); margin-top:4px;">%s Report · %s</div>
  </div>

  <div style="padding:24px;">
    %s

    <!-- Footer -->
    <div style="border-top:1px solid #e5e7eb; margin-top:24px; padding-top:16px; text-align:center;">
      <p style="font-size:12px; color:#9ca3af; line-height:1.6;">
        This report was automatically generated by Zenstats.<br>
        %s
      </p>
    </div>
  </div>
</body>
</html>`, d.DisplayName, d.ReportName, d.DateLabel, statsHTML, d.UnsubscribeNote)
}

func renderStatsSection(s *reportStats) string {
	// Metric card row
	cards := fmt.Sprintf(`
  <div style="margin-bottom:24px;">
    <table width="100%%" cellpadding="0" cellspacing="0" style="border-collapse:collapse;">
      <tr>
        %s
        %s
      </tr>
      <tr>
        %s
        %s
      </tr>
    </table>
  </div>`,
		renderMetricCard("VISITORS", s.Visitors, true),
		renderMetricCard("PAGEVIEWS", s.Pageviews, true),
		renderMetricCard("BOUNCE RATE", s.BounceRate, false),
		renderMetricCard("VISIT DURATION", s.VisitDuration, true),
	)

	// Top Sources
	sourcesTable := ""
	if len(s.TopSources) > 0 {
		sourcesTable = renderRankTable("Top Sources", s.TopSources)
	}

	// Top Pages
	pagesTable := ""
	if len(s.TopPages) > 0 {
		pagesTable = renderRankTable("Top Pages", s.TopPages)
	}

	return cards + sourcesTable + pagesTable
}

func renderMetricCard(label string, m metricWithChange, positiveIsGood bool) string {
	valueStr := formatLargeNumber(m.Value)
	if label == "BOUNCE RATE" {
		valueStr = fmt.Sprintf("%.0f%%", m.Value)
	} else if label == "VISIT DURATION" {
		valueStr = fmt.Sprintf("%.0fs", m.Value)
	}

	changeHTML := ""
	if m.Change != nil {
		r := math.Round(*m.Change*10) / 10 // Keep one decimal place
		color := changeColorClass(r, positiveIsGood)
		sign := ""
		if r > 0 {
			sign = "+"
		}
		changeHTML = fmt.Sprintf(`<div style="font-size:12px; color:%s; margin-top:2px;">%s%.0f%%</div>`,
			color, sign, r)
	} else {
		changeHTML = `<div style="font-size:12px; color:#888; margin-top:2px;">N/A</div>`
	}

	return fmt.Sprintf(`
        <td style="width:25%%; padding:8px; vertical-align:top; text-align:center;">
          <div style="font-size:10px; font-weight:700; color:#888; letter-spacing:0.5px;">%s</div>
          <div style="font-size:22px; font-weight:700; color:#1a1a2e; margin-top:4px;">%s</div>
          %s
        </td>`, label, valueStr, changeHTML)
}

func renderRankTable(title string, items []rankedItem) string {
	var rows strings.Builder
	for _, item := range items {
		rows.WriteString(fmt.Sprintf(`
        <tr>
          <td style="padding:6px 0; font-size:13px; max-width:300px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;">%s</td>
          <td style="padding:6px 0; font-size:13px; text-align:right; font-weight:600; white-space:nowrap;">%s</td>
        </tr>`, item.Name, formatLargeNumber(item.Visitors)))
	}

	return fmt.Sprintf(`
  <div style="margin-bottom:20px;">
    <div style="font-size:11px; font-weight:700; color:#888; letter-spacing:0.5px; margin-bottom:8px;">%s</div>
    <table width="100%%" cellpadding="0" cellspacing="0" style="border-collapse:collapse;">
      %s
    </table>
  </div>`, title, rows.String())
}

// ============================================================================
// Cron Registration + Test API
// ============================================================================

// RegisterReportCron registers the email report cron jobs.
// All cron expressions use UTC.
// Weekly: Monday 00:00 UTC (08:00 CST)
// Monthly: 1st of month 01:00 UTC (09:00 CST)
func RegisterReportCron() {
	cron := scheduler.GetCronManager()
	svc := GetEmailReportService()

	cron.AddJob("0 0 0 * * 1", svc.SendWeeklyReports, nil)
	cron.AddJob("0 0 1 1 * *", svc.SendMonthlyReports, nil)
	slog.Info("registered email report cron jobs (weekly Mon 00:00 UTC / monthly 1st 01:00 UTC)")
}

// BuildTestReportHTML generates a test report HTML for CLI use.
// interval: "weekly" or "monthly"
func BuildTestReportHTML(domain, interval string) string {
	svc := GetEmailReportService()
	ctx := context.Background()

	siteEnt, err := svc.siteService.GetSiteByDomain(ctx, domain)
	if err != nil {
		return fmt.Sprintf("<p>Site not found: %s</p>", domain)
	}

	ownerID, err := svc.siteService.GetSiteOwnerUserID(ctx, siteEnt.ID)
	if err != nil || ownerID == 0 {
		return "<p>Site owner not found</p>"
	}

	html, err := svc.buildReportHTML(ctx, siteEnt, ownerID, interval)
	if err != nil {
		return fmt.Sprintf("<p>Failed to build report: %v</p>", err)
	}
	return html
}

// SendTestReportMail sends a test report email for CLI use.
func SendTestReportMail(to, subject, html string) error {
	svc := GetEmailReportService()
	return svc.sendMail(to, subject, html)
}
