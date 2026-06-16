package imports

import (
	"fmt"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/pkg/response"
)

type ImportHandle struct {
	importService *service.ImportService
}

func NewImportHandle() *ImportHandle {
	return &ImportHandle{
		importService: service.GetImportService(),
	}
}

// Upload 上传 GA4 CSV 文件并导入到 ClickHouse
//
//	@Summary		导入 GA4 CSV 数据
//	@Description	上传 Google Analytics 4 导出的 CSV 文件，解析后写入 imported_* 表
//	@Tags			数据导入
//	@Security		BearerAuth
//	@Accept			multipart/form-data
//	@Produce		json
//	@Param			domain		path		string	true	"站点域名"
//	@Param			file		formData	file	true	"CSV 文件"
//	@Param			report_type	formData	string	false	"报告类型，不传则自动检测：visitors/pages/sources/browsers/os/devices/locations/entry_pages/exit_pages"
//	@Success		200			{object}	response.SuccessResponse{data=UploadResponse}
//	@Failure		400			{object}	response.ErrorResponse
//	@Failure		500			{object}	response.ErrorResponse
//	@Router			/sites/{domain}/import/upload [post]
func (h *ImportHandle) Upload() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteID := uint64(c.GetInt64("site_id"))

		file, header, err := c.Request.FormFile("file")
		if err != nil {
			response.Error(c, http.StatusBadRequest, fmt.Errorf("file required: %w", err))
			return
		}
		defer file.Close()

		if err := validateCSVFile(header); err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		reportType := c.DefaultPostForm("report_type", "")

		// If report type not provided, try auto-detection
		if reportType == "" {
			detected, ok := detectCSVReportType(file)
			if !ok {
				response.Error(c, http.StatusBadRequest, fmt.Errorf("unable to detect report type, please specify report_type"))
				return
			}
			reportType = detected
			// Reset file reader for re-reading
			file.Seek(0, 0)
		}

		importID := h.importService.NextImportID()

		rows, count, err := parseGA4CSV(file, siteID, importID, reportType)
		if err != nil {
			response.Error(c, http.StatusBadRequest, fmt.Errorf("csv parse error: %w", err))
			return
		}
		if count == 0 {
			response.Success(c, UploadResponse{
				ImportID:     importID,
				RowsImported: 0,
				ReportType:   reportType,
				Table:        h.importService.TableNameForReport(reportType),
			})
			return
		}

		table := h.importService.TableNameForReport(reportType)
		if err := h.importService.InsertByTable(c, table, rows); err != nil {
			response.Error(c, http.StatusInternalServerError, fmt.Errorf("insert error: %w", err))
			return
		}

		response.Success(c, UploadResponse{
			ImportID:     importID,
			RowsImported: count,
			ReportType:   reportType,
			Table:        table,
		})
	}
}

// GetAggregate 获取导入数据的聚合指标
//
//	@Summary		导入数据聚合
//	@Description	从 imported_visitors 表查询聚合指标
//	@Tags			数据导入
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			domain	path		string	true	"站点域名"
//	@Param			from	query		string	false	"开始日期 YYYY-MM-DD，默认30天前"
//	@Param			to		query		string	false	"结束日期 YYYY-MM-DD，默认今天"
//	@Success		200		{object}	response.SuccessResponse{data=AggregateResponse}
//	@Router			/sites/{domain}/import/aggregate [get]
func (h *ImportHandle) GetAggregate() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteID := uint64(c.GetInt64("site_id"))

		from, to, err := parseDateRange(c.Query("from"), c.Query("to"))
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		result, err := h.importService.QueryAggregate(c, siteID, from, to)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, AggregateResponse{
			Visitors:      result.Visitors,
			Pageviews:     result.Pageviews,
			Visits:        result.Visits,
			BounceRate:    result.BounceRate,
			VisitDuration: result.VisitDuration,
			ViewsPerVisit: result.ViewsPerVisit,
		})
	}
}

// GetBreakdown 获取导入数据的维度细分
//
//	@Summary		导入数据细分
//	@Description	从 imported_* 表按维度查询细分数据
//	@Tags			数据导入
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			domain		path		string	true	"站点域名"
//	@Param			from		query		string	false	"开始日期 YYYY-MM-DD"
//	@Param			to			query		string	false	"结束日期 YYYY-MM-DD"
//	@Param			property	query		string	true	"维度：visit:source, visit:country, visit:browser, visit:os, visit:device, visit:entry_page, visit:exit_page, event:page, event:hostname"
//	@Param			limit		query		int		false	"返回条数限制"	default(9)
//	@Param			page		query		int		false	"页码"	default(1)
//	@Success		200			{object}	response.SuccessResponse{data=BreakdownResponse}
//	@Router			/sites/{domain}/import/breakdown [get]
func (h *ImportHandle) GetBreakdown() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteID := uint64(c.GetInt64("site_id"))

		from, to, err := parseDateRange(c.Query("from"), c.Query("to"))
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		property := c.Query("property")
		if property == "" {
			response.Error(c, http.StatusBadRequest, fmt.Errorf("property is required"))
			return
		}

		limit := 9
		if l := c.Query("limit"); l != "" {
			if parsed, err := parseInt(l); err == nil && parsed > 0 && parsed <= 100 {
				limit = parsed
			}
		}

		page := 1
		if p := c.Query("page"); p != "" {
			if parsed, err := parseInt(p); err == nil && parsed > 0 {
				page = parsed
			}
		}
		offset := (page - 1) * limit

		rows, err := h.importService.QueryBreakdown(c, siteID, from, to, property, limit, offset)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		data := make([]BreakdownRow, 0, len(rows))
		for _, r := range rows {
			data = append(data, BreakdownRow{
				Name:      r.Name,
				Visitors:  r.Visitors,
				Pageviews: r.Pageviews,
			})
		}

		response.Success(c, BreakdownResponse{Data: data})
	}
}

// GetTimeSeries 获取导入数据的时间序列
//
//	@Summary		导入数据时间序列
//	@Description	从 imported_visitors 表按日查询时间序列
//	@Tags			数据导入
//	@Security		BearerAuth
//	@Security		APIKeyAuth
//	@Produce		json
//	@Param			domain	path		string	true	"站点域名"
//	@Param			from	query		string	false	"开始日期 YYYY-MM-DD"
//	@Param			to		query		string	false	"结束日期 YYYY-MM-DD"
//	@Success		200		{object}	response.SuccessResponse{data=[]TimeSeriesPoint}
//	@Router			/sites/{domain}/import/timeseries [get]
func (h *ImportHandle) GetTimeSeries() gin.HandlerFunc {
	return func(c *gin.Context) {
		siteID := uint64(c.GetInt64("site_id"))

		from, to, err := parseDateRange(c.Query("from"), c.Query("to"))
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		points, err := h.importService.QueryTimeSeries(c, siteID, from, to)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		data := make([]TimeSeriesPoint, 0, len(points))
		for _, p := range points {
			data = append(data, TimeSeriesPoint{
				Date:      p.Date,
				Visitors:  p.Visitors,
				Pageviews: p.Pageviews,
			})
		}

		response.Success(c, data)
	}
}

func validateCSVFile(header *multipart.FileHeader) error {
	if header == nil {
		return fmt.Errorf("no file provided")
	}
	if header.Size > 50*1024*1024 {
		return fmt.Errorf("file too large (max 50MB)")
	}
	if header.Size == 0 {
		return fmt.Errorf("empty file")
	}
	return nil
}

func parseDateRange(fromStr, toStr string) (time.Time, time.Time, error) {
	to := time.Now().UTC().Truncate(24 * time.Hour)
	from := to.AddDate(0, -1, 0)

	if fromStr != "" {
		t, err := time.Parse("2006-01-02", fromStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid from date: %s", fromStr)
		}
		from = t
	}
	if toStr != "" {
		t, err := time.Parse("2006-01-02", toStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid to date: %s", toStr)
		}
		to = t.Add(24*time.Hour - time.Second)
	}

	if from.After(to) {
		return time.Time{}, time.Time{}, fmt.Errorf("from date must be before to date")
	}

	return from, to, nil
}

func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
