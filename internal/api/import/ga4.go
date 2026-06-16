package imports

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/zenstats/zenstats/internal/store/clickhouse/models"
)

type reportTable struct {
	ReportType string
	TableName  string
	Columns    []ga4ColumnDef
}

type ga4ColumnDef struct {
	ga4Names []string
}

var ga4Reports = map[string]reportTable{
	"visitors": {
		ReportType: "visitors",
		TableName:  "imported_visitors",
		Columns: []ga4ColumnDef{
			{ga4Names: []string{"Date"}},
			{ga4Names: []string{"Users", "Total users", "Active users"}},
			{ga4Names: []string{"Views", "Screen views", "Event count"}},
			{ga4Names: []string{"Sessions", "User sessions"}},
		},
	},
	"pages": {
		ReportType: "pages",
		TableName:  "imported_pages",
		Columns: []ga4ColumnDef{
			{ga4Names: []string{"Date"}},
			{ga4Names: []string{"Page path and screen class", "Page path", "Page"}},
			{ga4Names: []string{"Views", "Screen views", "Item views"}},
			{ga4Names: []string{"Users", "Total users"}},
			{ga4Names: []string{"Events per session", "Event count"}},
		},
	},
	"sources": {
		ReportType: "sources",
		TableName:  "imported_sources",
		Columns: []ga4ColumnDef{
			{ga4Names: []string{"Date"}},
			{ga4Names: []string{"Session source", "First user source", "Source"}},
			{ga4Names: []string{"Session medium", "First user medium", "Medium"}},
			{ga4Names: []string{"Session campaign", "First user campaign", "Campaign"}},
			{ga4Names: []string{"Users", "Total users"}},
			{ga4Names: []string{"Sessions", "User sessions"}},
		},
	},
	"browsers": {
		ReportType: "browsers",
		TableName:  "imported_browsers",
		Columns: []ga4ColumnDef{
			{ga4Names: []string{"Date"}},
			{ga4Names: []string{"Browser"}},
			{ga4Names: []string{"Users", "Total users"}},
			{ga4Names: []string{"Sessions", "User sessions"}},
		},
	},
	"os": {
		ReportType: "os",
		TableName:  "imported_operating_systems",
		Columns: []ga4ColumnDef{
			{ga4Names: []string{"Date"}},
			{ga4Names: []string{"Operating system", "OS"}},
			{ga4Names: []string{"Users", "Total users"}},
			{ga4Names: []string{"Sessions", "User sessions"}},
			{ga4Names: []string{"Operating system version", "OS version"}},
		},
	},
	"devices": {
		ReportType: "devices",
		TableName:  "imported_devices",
		Columns: []ga4ColumnDef{
			{ga4Names: []string{"Date"}},
			{ga4Names: []string{"Device category", "Device"}},
			{ga4Names: []string{"Users", "Total users"}},
			{ga4Names: []string{"Sessions", "User sessions"}},
		},
	},
	"locations": {
		ReportType: "locations",
		TableName:  "imported_locations",
		Columns: []ga4ColumnDef{
			{ga4Names: []string{"Date"}},
			{ga4Names: []string{"Country"}},
			{ga4Names: []string{"Region", "Region / State"}},
			{ga4Names: []string{"City"}},
			{ga4Names: []string{"Users", "Total users"}},
			{ga4Names: []string{"Sessions", "User sessions"}},
		},
	},
	"entry_pages": {
		ReportType: "entry_pages",
		TableName:  "imported_entry_pages",
		Columns: []ga4ColumnDef{
			{ga4Names: []string{"Date"}},
			{ga4Names: []string{"Landing page", "Entry page", "Entry page path"}},
			{ga4Names: []string{"Users", "Total users"}},
			{ga4Names: []string{"Sessions", "User sessions"}},
			{ga4Names: []string{"Entrances"}},
		},
	},
	"exit_pages": {
		ReportType: "exit_pages",
		TableName:  "imported_exit_pages",
		Columns: []ga4ColumnDef{
			{ga4Names: []string{"Date"}},
			{ga4Names: []string{"Exit page", "Exit page path"}},
			{ga4Names: []string{"Users", "Total users"}},
			{ga4Names: []string{"Exits"}},
		},
	},
}

func detectReportType(header []string) (string, bool) {
	lookup := make(map[string]int)
	for i, col := range header {
		lookup[strings.TrimSpace(col)] = i
	}

	score := func(reportType string) int {
		cfg, ok := ga4Reports[reportType]
		if !ok {
			return 0
		}
		s := 0
		for _, def := range cfg.Columns {
			for _, name := range def.ga4Names {
				if _, found := lookup[name]; found {
					s++
					break
				}
			}
		}
		return s
	}

	best := ""
	bestScore := 0
	for rt := range ga4Reports {
		if s := score(rt); s > bestScore {
			bestScore = s
			best = rt
		}
	}

	if bestScore < 2 {
		return "", false
	}
	return best, true
}

func parseGA4CSV(r io.Reader, siteID uint64, importID uint64, reportType string) (any, int, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true
	reader.LazyQuotes = true

	header, err := reader.Read()
	if err != nil {
		return nil, 0, fmt.Errorf("read csv header: %w", err)
	}

	colIndex := make(map[string]int)
	for i, h := range header {
		colIndex[cleanHeaderName(h)] = i
	}

	getCol := func(names []string) (int, bool) {
		for _, n := range names {
			if idx, ok := colIndex[n]; ok {
				return idx, true
			}
		}
		return 0, false
	}

	switch reportType {
	case "visitors":
		return parseVisitorsCSV(reader, getCol, siteID, importID)
	case "pages":
		return parsePagesCSV(reader, getCol, siteID, importID)
	case "sources":
		return parseSourcesCSV(reader, getCol, siteID, importID)
	case "browsers":
		return parseBrowsersCSV(reader, getCol, siteID, importID)
	case "os":
		return parseOSCSV(reader, getCol, siteID, importID)
	case "devices":
		return parseDevicesCSV(reader, getCol, siteID, importID)
	case "locations":
		return parseLocationsCSV(reader, getCol, siteID, importID)
	case "entry_pages":
		return parseEntryPagesCSV(reader, getCol, siteID, importID)
	case "exit_pages":
		return parseExitPagesCSV(reader, getCol, siteID, importID)
	default:
		return nil, 0, fmt.Errorf("unsupported report type: %s", reportType)
	}
}

func parseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, fmtStr := range []string{"2006-01-02", "20060102", "01/02/2006", "Jan 2, 2006"} {
		if t, err := time.Parse(fmtStr, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse date: %s", s)
}

func parseUint64(s string) uint64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return 0
	}
	s = strings.ReplaceAll(s, ",", "")
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

func readAllCSVRows(reader *csv.Reader) ([][]string, error) {
	var rows [][]string
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		rows = append(rows, record)
	}
	return rows, nil
}

func parseVisitorsCSV(reader *csv.Reader, getCol func([]string) (int, bool), siteID uint64, importID uint64) (any, int, error) {
	rows, err := readAllCSVRows(reader)
	if err != nil {
		return nil, 0, err
	}

	dateIdx, _ := getCol([]string{"Date"})
	usersIdx, _ := getCol([]string{"Users", "Total users", "Active users"})
	viewsIdx, _ := getCol([]string{"Views", "Screen views", "Event count"})
	sessionsIdx, _ := getCol([]string{"Sessions", "User sessions"})

	var result []*models.ImportedVisitors
	for _, row := range rows {
		date, err := parseDate(row[dateIdx])
		if err != nil {
			continue
		}
		record := &models.ImportedVisitors{
			SiteID:        siteID,
			Date:          date,
			Visitors:      parseUint64(getVal(row, usersIdx)),
			Pageviews:     parseUint64(getVal(row, viewsIdx)),
			Visits:        parseUint64(getVal(row, sessionsIdx)),
			VisitDuration: 0,
			Bounces:       0,
			ImportID:      importID,
		}
		result = append(result, record)
	}
	return result, len(result), nil
}

func parsePagesCSV(reader *csv.Reader, getCol func([]string) (int, bool), siteID uint64, importID uint64) (any, int, error) {
	rows, err := readAllCSVRows(reader)
	if err != nil {
		return nil, 0, err
	}

	dateIdx, _ := getCol([]string{"Date"})
	pageIdx, _ := getCol([]string{"Page path and screen class", "Page path", "Page"})
	viewsIdx, _ := getCol([]string{"Views", "Screen views", "Item views"})
	usersIdx, _ := getCol([]string{"Users", "Total users"})

	var result []*models.ImportedPage
	for _, row := range rows {
		date, err := parseDate(row[dateIdx])
		if err != nil {
			continue
		}
		page := strings.TrimSpace(row[pageIdx])
		if page == "" {
			continue
		}
		record := &models.ImportedPage{
			SiteID:    siteID,
			Date:      date,
			Hostname:  "",
			Page:      page,
			Visitors:  parseUint64(getVal(row, usersIdx)),
			Pageviews: parseUint64(getVal(row, viewsIdx)),
			ImportID:  importID,
		}
		result = append(result, record)
	}
	return result, len(result), nil
}

func parseSourcesCSV(reader *csv.Reader, getCol func([]string) (int, bool), siteID uint64, importID uint64) (any, int, error) {
	rows, err := readAllCSVRows(reader)
	if err != nil {
		return nil, 0, err
	}

	dateIdx, _ := getCol([]string{"Date"})
	sourceIdx, _ := getCol([]string{"Session source", "First user source", "Source"})
	mediumIdx, _ := getCol([]string{"Session medium", "First user medium", "Medium"})
	campaignIdx, _ := getCol([]string{"Session campaign", "First user campaign", "Campaign"})
	usersIdx, _ := getCol([]string{"Users", "Total users"})
	sessionsIdx, _ := getCol([]string{"Sessions", "User sessions"})

	var result []*models.ImportedSource
	for _, row := range rows {
		date, err := parseDate(row[dateIdx])
		if err != nil {
			continue
		}
		source := strings.TrimSpace(getVal(row, sourceIdx))
		if source == "" {
			continue
		}
		record := &models.ImportedSource{
			SiteID:      siteID,
			Date:        date,
			Source:      source,
			UtmMedium:   strings.TrimSpace(getVal(row, mediumIdx)),
			UtmCampaign: strings.TrimSpace(getVal(row, campaignIdx)),
			Visitors:    parseUint64(getVal(row, usersIdx)),
			Visits:      parseUint64(getVal(row, sessionsIdx)),
			Pageviews:   0,
			ImportID:    importID,
		}
		result = append(result, record)
	}
	return result, len(result), nil
}

func parseBrowsersCSV(reader *csv.Reader, getCol func([]string) (int, bool), siteID uint64, importID uint64) (any, int, error) {
	rows, err := readAllCSVRows(reader)
	if err != nil {
		return nil, 0, err
	}

	dateIdx, _ := getCol([]string{"Date"})
	browserIdx, _ := getCol([]string{"Browser"})
	usersIdx, _ := getCol([]string{"Users", "Total users"})
	sessionsIdx, _ := getCol([]string{"Sessions", "User sessions"})

	var result []*models.ImportedBrowser
	for _, row := range rows {
		date, err := parseDate(row[dateIdx])
		if err != nil {
			continue
		}
		browser := strings.TrimSpace(getVal(row, browserIdx))
		if browser == "" {
			continue
		}
		record := &models.ImportedBrowser{
			SiteID:    siteID,
			Date:      date,
			Browser:   browser,
			Visitors:  parseUint64(getVal(row, usersIdx)),
			Visits:    parseUint64(getVal(row, sessionsIdx)),
			Pageviews: 0,
			ImportID:  importID,
		}
		result = append(result, record)
	}
	return result, len(result), nil
}

func parseOSCSV(reader *csv.Reader, getCol func([]string) (int, bool), siteID uint64, importID uint64) (any, int, error) {
	rows, err := readAllCSVRows(reader)
	if err != nil {
		return nil, 0, err
	}

	dateIdx, _ := getCol([]string{"Date"})
	osIdx, _ := getCol([]string{"Operating system", "OS"})
	usersIdx, _ := getCol([]string{"Users", "Total users"})
	sessionsIdx, _ := getCol([]string{"Sessions", "User sessions"})
	osVerIdx, ok := getCol([]string{"Operating system version", "OS version"})

	var result []*models.ImportedOS
	for _, row := range rows {
		date, err := parseDate(row[dateIdx])
		if err != nil {
			continue
		}
		os := strings.TrimSpace(getVal(row, osIdx))
		if os == "" {
			continue
		}
		record := &models.ImportedOS{
			SiteID:                 siteID,
			Date:                   date,
			OperatingSystem:        os,
			Visitors:               parseUint64(getVal(row, usersIdx)),
			Visits:                 parseUint64(getVal(row, sessionsIdx)),
			Pageviews:              0,
			ImportID:               importID,
			OperatingSystemVersion: strings.TrimSpace(getValByOpt(row, osVerIdx, ok)),
		}
		result = append(result, record)
	}
	return result, len(result), nil
}

func parseDevicesCSV(reader *csv.Reader, getCol func([]string) (int, bool), siteID uint64, importID uint64) (any, int, error) {
	rows, err := readAllCSVRows(reader)
	if err != nil {
		return nil, 0, err
	}

	dateIdx, _ := getCol([]string{"Date"})
	deviceIdx, _ := getCol([]string{"Device category", "Device"})
	usersIdx, _ := getCol([]string{"Users", "Total users"})
	sessionsIdx, _ := getCol([]string{"Sessions", "User sessions"})

	var result []*models.ImportedDevice
	for _, row := range rows {
		date, err := parseDate(row[dateIdx])
		if err != nil {
			continue
		}
		device := strings.TrimSpace(getVal(row, deviceIdx))
		if device == "" {
			continue
		}
		record := &models.ImportedDevice{
			SiteID:    siteID,
			Date:      date,
			Device:    device,
			Visitors:  parseUint64(getVal(row, usersIdx)),
			Visits:    parseUint64(getVal(row, sessionsIdx)),
			Pageviews: 0,
			ImportID:  importID,
		}
		result = append(result, record)
	}
	return result, len(result), nil
}

func parseLocationsCSV(reader *csv.Reader, getCol func([]string) (int, bool), siteID uint64, importID uint64) (any, int, error) {
	rows, err := readAllCSVRows(reader)
	if err != nil {
		return nil, 0, err
	}

	dateIdx, _ := getCol([]string{"Date"})
	countryIdx, _ := getCol([]string{"Country"})
	regionIdx, _ := getCol([]string{"Region", "Region / State"})
	_, _ = getCol([]string{"City"})
	usersIdx, _ := getCol([]string{"Users", "Total users"})
	sessionsIdx, _ := getCol([]string{"Sessions", "User sessions"})

	var result []*models.ImportedLocation
	for _, row := range rows {
		date, err := parseDate(row[dateIdx])
		if err != nil {
			continue
		}
		country := strings.TrimSpace(getVal(row, countryIdx))
		if country == "" {
			continue
		}
		record := &models.ImportedLocation{
			SiteID:    siteID,
			Date:      date,
			Country:   country,
			Region:    strings.TrimSpace(getVal(row, regionIdx)),
			City:      0,
			Visitors:  parseUint64(getVal(row, usersIdx)),
			Visits:    parseUint64(getVal(row, sessionsIdx)),
			Pageviews: 0,
			ImportID:  importID,
		}
		result = append(result, record)
	}
	return result, len(result), nil
}

func parseEntryPagesCSV(reader *csv.Reader, getCol func([]string) (int, bool), siteID uint64, importID uint64) (any, int, error) {
	rows, err := readAllCSVRows(reader)
	if err != nil {
		return nil, 0, err
	}

	dateIdx, _ := getCol([]string{"Date"})
	entryIdx, _ := getCol([]string{"Landing page", "Entry page", "Entry page path"})
	usersIdx, _ := getCol([]string{"Users", "Total users"})
	sessionsIdx, _ := getCol([]string{"Sessions", "User sessions"})
	entrancesIdx, _ := getCol([]string{"Entrances"})

	var result []*models.ImportedEntryPage
	for _, row := range rows {
		date, err := parseDate(row[dateIdx])
		if err != nil {
			continue
		}
		entry := strings.TrimSpace(getVal(row, entryIdx))
		if entry == "" {
			continue
		}
		record := &models.ImportedEntryPage{
			SiteID:     siteID,
			Date:       date,
			EntryPage:  entry,
			Visitors:   parseUint64(getVal(row, usersIdx)),
			Pageviews:  parseUint64(getVal(row, sessionsIdx)),
			Entrances:  parseUint64(getVal(row, entrancesIdx)),
			VisitDuration: 0,
			ImportID:   importID,
		}
		result = append(result, record)
	}
	return result, len(result), nil
}

func parseExitPagesCSV(reader *csv.Reader, getCol func([]string) (int, bool), siteID uint64, importID uint64) (any, int, error) {
	rows, err := readAllCSVRows(reader)
	if err != nil {
		return nil, 0, err
	}

	dateIdx, _ := getCol([]string{"Date"})
	exitIdx, _ := getCol([]string{"Exit page", "Exit page path"})
	usersIdx, _ := getCol([]string{"Users", "Total users"})
	exitsIdx, _ := getCol([]string{"Exits"})

	var result []*models.ImportedExitPage
	for _, row := range rows {
		date, err := parseDate(row[dateIdx])
		if err != nil {
			continue
		}
		exit := strings.TrimSpace(getVal(row, exitIdx))
		if exit == "" {
			continue
		}
		record := &models.ImportedExitPage{
			SiteID:    siteID,
			Date:      date,
			ExitPage:  exit,
			Visitors:  parseUint64(getVal(row, usersIdx)),
			Exits:     parseUint64(getVal(row, exitsIdx)),
			Pageviews: 0,
			ImportID:  importID,
		}
		result = append(result, record)
	}
	return result, len(result), nil
}

func detectCSVReportType(r io.ReadSeeker) (string, bool) {
	reader := csv.NewReader(r)
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err != nil {
		return "", false
	}

	cleaned := make([]string, len(header))
	for i, h := range header {
		cleaned[i] = cleanHeaderName(h)
	}

	return detectReportType(cleaned)
}

func cleanHeaderName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimLeft(s, "\ufeff\xef\xbb\xbf")
	s = strings.TrimSpace(s)
	return s
}

func getVal(row []string, idx int) string {
	if idx >= 0 && idx < len(row) {
		return row[idx]
	}
	return ""
}

func getValByOpt(row []string, idx int, ok bool) string {
	if ok {
		return getVal(row, idx)
	}
	return ""
}
