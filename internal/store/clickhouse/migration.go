package clickhouse

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
)

type Migration struct {
	conn clickhouse.Conn
}

func NewMigration() *Migration {
	return &Migration{conn: GetConnection()}
}

func (m *Migration) Run() error {
	ctx := context.Background()

	// 1. 基础表创建（幂等 CREATE TABLE IF NOT EXISTS）
	err := m.ensureBaseTables(ctx)
	if err != nil {
		return err
	}

	// 2. 在线 schema 变更（幂等 ALTER TABLE，已有表升级用）
	err = m.runOnlineMigrations(ctx)
	if err != nil {
		slog.Error("online migration failed", "error", err)
	}

	slog.Info("migration completed")
	return nil
}

// RunOnline 仅执行在线 schema 变更（不创建表），用于升级已有部署。
// 可通过 CLI 命令或 entrypoint 调用：/app/zenstats migrate-clickhouse
func (m *Migration) RunOnline() error {
	ctx := context.Background()
	slog.Info("running online ClickHouse migrations")
	return m.runOnlineMigrations(ctx)
}

func (m *Migration) ensureBaseTables(ctx context.Context) error {
	err := m.ensureLocationDataTable(ctx)
	if err != nil {
		slog.Error("ensure location_data table failed", "error", err)
	}
	err = m.ensureLocationDataDictTable(ctx)
	if err != nil {
		slog.Error("ensure location_data_dict table failed", "error", err)
	}
	err = m.ensureImportedVisitorsTable(ctx)
	if err != nil {
		slog.Error("ensure imported_visitors table failed", "error", err)
	}
	err = m.ensureImportedSourceTable(ctx)
	if err != nil {
		slog.Error("ensure imported_source table failed", "error", err)
	}

	err = m.ensureImportedPagesTable(ctx)
	if err != nil {
		slog.Error("ensure imported_pages table failed", "error", err)
	}
	err = m.ensureImportedOperatingSystemsTable(ctx)
	if err != nil {
		slog.Error("ensure imported_operating_systems table failed", "error", err)
	}
	err = m.ensureImportedlocationsTable(ctx)
	if err != nil {
		slog.Error("ensure imported_locations table failed", "error", err)
	}
	err = m.ensureImportedExitPageTable(ctx)
	if err != nil {
		slog.Error("ensure imported_exit_pages table failed", "error", err)
	}
	err = m.ensureImportedEntryPageTable(ctx)
	if err != nil {
		slog.Error("ensure imported_entry_pages table failed", "error", err)
	}
	err = m.ensureImportedDevicesTable(ctx)
	if err != nil {
		slog.Error("ensure imported_devices table failed", "error", err)
	}
	err = m.ensureImportedCustomEventsTable(ctx)
	if err != nil {
		slog.Error("ensure imported_custom_events table failed", "error", err)
	}
	err = m.ensureImportedBrowsersTable(ctx)
	if err != nil {
		slog.Error("ensure imported_browsers table failed", "error", err)
	}

	err = m.ensureSessionsTable(ctx)
	if err != nil {
		slog.Error("ensure sessions table failed", "error", err)
	}
	err = m.ensureEventsTable(ctx)
	if err != nil {
		slog.Error("ensure events table failed", "error", err)
	}
	return nil
}

// runOnlineMigrations 执行在线 schema 变更（ALTER TABLE），用于升级已有部署。
// 所有语句均为幂等设计，可安全重复执行。
func (m *Migration) runOnlineMigrations(ctx context.Context) error {
	bt := "`" // backtick shorthand

	// ========== events 表 ==========
	eventsAlters := []string{
		// device 列重命名为 screen_size（如果旧表列名是 device）
		fmt.Sprintf(`ALTER TABLE zenstats_events_db.events RENAME COLUMN IF EXISTS %sdevice%s TO %sscreen_size%s`, bt, bt, bt, bt),
		// 兼容别名
		fmt.Sprintf(`ALTER TABLE zenstats_events_db.events ADD COLUMN IF NOT EXISTS %sdevice%s LowCardinality(String) ALIAS screen_size`, bt, bt),
		// user_agent 类型变更
		fmt.Sprintf(`ALTER TABLE zenstats_events_db.events MODIFY COLUMN IF EXISTS %suser_agent%s String CODEC(ZSTD(3))`, bt, bt),
	}

	// ========== sessions 表 ==========
	sessionsAlters := []string{
		// 列重命名
		fmt.Sprintf(`ALTER TABLE zenstats_events_db.sessions RENAME COLUMN IF EXISTS %sdevice%s TO %sscreen_size%s`, bt, bt, bt, bt),
		// version 列
		fmt.Sprintf(`ALTER TABLE zenstats_events_db.sessions ADD COLUMN IF NOT EXISTS %sversion%s UInt64`, bt, bt),
		// 兼容别名
		fmt.Sprintf(`ALTER TABLE zenstats_events_db.sessions ADD COLUMN IF NOT EXISTS %sdevice%s LowCardinality(String) ALIAS screen_size`, bt, bt),
		fmt.Sprintf(`ALTER TABLE zenstats_events_db.sessions ADD COLUMN IF NOT EXISTS %sentry_page_hostname%s String ALIAS hostname`, bt, bt),
		// user_agent 类型变更
		fmt.Sprintf(`ALTER TABLE zenstats_events_db.sessions MODIFY COLUMN IF EXISTS %suser_agent%s String CODEC(ZSTD(3))`, bt, bt),
	}

	for _, sql := range eventsAlters {
		if err := m.conn.Exec(ctx, sql); err != nil {
			slog.Warn("online migration (events)", "sql", sql, "error", err)
		}
	}
	for _, sql := range sessionsAlters {
		if err := m.conn.Exec(ctx, sql); err != nil {
			slog.Warn("online migration (sessions)", "sql", sql, "error", err)
		}
	}

	slog.Info("online ClickHouse migrations applied")
	return nil
}

func (m *Migration) Refresh() error {
	ctx := context.Background()
	err := m.dropAllTable(ctx)
	if err != nil {
		return err
	}

	err = m.Run()

	return err
}

func (m *Migration) dropAllTable(ctx context.Context) error {
	tables := []string{"location_data", "location_data_dict", "events", "sessions"}
	for _, table := range tables {
		err := m.dropTable(ctx, table)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *Migration) dropTable(ctx context.Context, table string) error {
	sql := fmt.Sprintf("DROP TABLE IF EXISTS zenstats_events_db.%s;", table)

	return m.conn.Exec(ctx, sql)
}

func (m *Migration) ensureLocationDataTable(ctx context.Context) error {
	sql := `CREATE TABLE IF NOT EXISTS zenstats_events_db.location_data
(
    {{backtick}}type{{backtick}} LowCardinality(String),
    {{backtick}}id{{backtick}} String,
    {{backtick}}name{{backtick}} String
)
ENGINE = MergeTree
ORDER BY (type, id)
SETTINGS index_granularity = 128;
`
	return m.conn.Exec(ctx, strings.ReplaceAll(sql, "{{backtick}}", "`"))
}

func (m *Migration) ensureLocationDataDictTable(ctx context.Context) error {
	sql := `CREATE DICTIONARY IF NOT EXISTS zenstats_events_db.location_data_dict
(
    {{backtick}}type{{backtick}} String,
    {{backtick}}id{{backtick}} String,
    {{backtick}}name{{backtick}} String
)
PRIMARY KEY type, id
SOURCE(CLICKHOUSE(TABLE location_data DB 'zenstats_events_db'))
LIFETIME(MIN 0 MAX 0)
LAYOUT(COMPLEX_KEY_CACHE(SIZE_IN_CELLS 500000));`

	return m.conn.Exec(ctx, strings.ReplaceAll(sql, "{{backtick}}", "`"))
}

func (m *Migration) ensureImportedVisitorsTable(ctx context.Context) error {
	sql := `CREATE TABLE IF NOT EXISTS zenstats_events_db.imported_visitors
(
    {{backtick}}site_id{{backtick}} UInt64,
    {{backtick}}date{{backtick}} Date,
    {{backtick}}visitors{{backtick}} UInt64,
    {{backtick}}pageviews{{backtick}} UInt64,
    {{backtick}}bounces{{backtick}} UInt64,
    {{backtick}}visits{{backtick}} UInt64,
    {{backtick}}visit_duration{{backtick}} UInt64,
    {{backtick}}import_id{{backtick}} UInt64
)
ENGINE = MergeTree
ORDER BY (site_id, date)
SETTINGS index_granularity = 8192, replicated_deduplication_window = 0;`

	return m.conn.Exec(ctx, strings.ReplaceAll(sql, "{{backtick}}", "`"))
}

func (m *Migration) ensureImportedSourceTable(ctx context.Context) error {
	sql := `CREATE TABLE IF NOT EXISTS zenstats_events_db.imported_sources
(
    {{backtick}}site_id{{backtick}} UInt64,
    {{backtick}}date{{backtick}} Date,
    {{backtick}}source{{backtick}} String,
    {{backtick}}utm_medium{{backtick}} String,
    {{backtick}}utm_campaign{{backtick}} String,
    {{backtick}}utm_content{{backtick}} String,
    {{backtick}}utm_term{{backtick}} String,
    {{backtick}}visitors{{backtick}} UInt64,
    {{backtick}}visits{{backtick}} UInt64,
    {{backtick}}visit_duration{{backtick}} UInt64,
    {{backtick}}bounces{{backtick}} UInt32,
    {{backtick}}import_id{{backtick}} UInt64,
    {{backtick}}pageviews{{backtick}} UInt64,
    {{backtick}}referrer{{backtick}} String,
    {{backtick}}utm_source{{backtick}} String
)
ENGINE = MergeTree
ORDER BY (site_id, date, source)
SETTINGS index_granularity = 8192, replicated_deduplication_window = 0;`

	return m.conn.Exec(ctx, strings.ReplaceAll(sql, "{{backtick}}", "`"))
}
func (m *Migration) ensureImportedPagesTable(ctx context.Context) error {
	sql := `CREATE TABLE IF NOT EXISTS zenstats_events_db.imported_pages
(
    {{backtick}}site_id{{backtick}} UInt64,
    {{backtick}}date{{backtick}} Date,
    {{backtick}}hostname{{backtick}} String,
    {{backtick}}page{{backtick}} String,
    {{backtick}}visitors{{backtick}} UInt64,
    {{backtick}}pageviews{{backtick}} UInt64,
    {{backtick}}exits{{backtick}} UInt64,
    {{backtick}}time_on_page{{backtick}} UInt64,
    {{backtick}}import_id{{backtick}} UInt64,
    {{backtick}}visits{{backtick}} UInt64,
    {{backtick}}active_visitors{{backtick}} UInt64
)
ENGINE = MergeTree
ORDER BY (site_id, date, hostname, page)
SETTINGS index_granularity = 8192, replicated_deduplication_window = 0;`

	return m.conn.Exec(ctx, strings.ReplaceAll(sql, "{{backtick}}", "`"))
}
func (m *Migration) ensureImportedOperatingSystemsTable(ctx context.Context) error {
	sql := `CREATE TABLE IF NOT EXISTS zenstats_events_db.imported_operating_systems
(
    {{backtick}}site_id{{backtick}} UInt64,
    {{backtick}}date{{backtick}} Date,
    {{backtick}}operating_system{{backtick}} String,
    {{backtick}}visitors{{backtick}} UInt64,
    {{backtick}}visits{{backtick}} UInt64,
    {{backtick}}visit_duration{{backtick}} UInt64,
    {{backtick}}bounces{{backtick}} UInt32,
    {{backtick}}import_id{{backtick}} UInt64,
    {{backtick}}pageviews{{backtick}} UInt64,
    {{backtick}}operating_system_version{{backtick}} String
)
ENGINE = MergeTree
ORDER BY (site_id, date, operating_system)
SETTINGS index_granularity = 8192, replicated_deduplication_window = 0;`

	return m.conn.Exec(ctx, strings.ReplaceAll(sql, "{{backtick}}", "`"))
}
func (m *Migration) ensureImportedlocationsTable(ctx context.Context) error {
	sql := `CREATE TABLE IF NOT EXISTS zenstats_events_db.imported_locations
(
    {{backtick}}site_id{{backtick}} UInt64,
    {{backtick}}date{{backtick}} Date,
    {{backtick}}country{{backtick}} String,
    {{backtick}}region{{backtick}} String,
    {{backtick}}city{{backtick}} UInt64,
    {{backtick}}visitors{{backtick}} UInt64,
    {{backtick}}visits{{backtick}} UInt64,
    {{backtick}}visit_duration{{backtick}} UInt64,
    {{backtick}}bounces{{backtick}} UInt32,
    {{backtick}}import_id{{backtick}} UInt64,
    {{backtick}}pageviews{{backtick}} UInt64,
    {{backtick}}country_name{{backtick}} String ALIAS dictGet('zenstats_events_db.location_data_dict', 'name', ('country', country)),
    {{backtick}}region_name{{backtick}} String ALIAS dictGet('zenstats_events_db.location_data_dict', 'name', ('subdivision', region)),
    {{backtick}}city_name{{backtick}} String ALIAS dictGet('zenstats_events_db.location_data_dict', 'name', ('city', city))
)
ENGINE = MergeTree
ORDER BY (site_id, date, country, region, city)
SETTINGS index_granularity = 8192, replicated_deduplication_window = 0;`

	return m.conn.Exec(ctx, strings.ReplaceAll(sql, "{{backtick}}", "`"))
}
func (m *Migration) ensureImportedExitPageTable(ctx context.Context) error {
	sql := `CREATE TABLE IF NOT EXISTS zenstats_events_db.imported_exit_pages
(
    {{backtick}}site_id{{backtick}} UInt64,
    {{backtick}}date{{backtick}} Date,
    {{backtick}}exit_page{{backtick}} String,
    {{backtick}}visitors{{backtick}} UInt64,
    {{backtick}}exits{{backtick}} UInt64,
    {{backtick}}import_id{{backtick}} UInt64,
    {{backtick}}pageviews{{backtick}} UInt64,
    {{backtick}}bounces{{backtick}} UInt32,
    {{backtick}}visit_duration{{backtick}} UInt64
)
ENGINE = MergeTree
ORDER BY (site_id, date, exit_page)
SETTINGS index_granularity = 8192, replicated_deduplication_window = 0;`

	return m.conn.Exec(ctx, strings.ReplaceAll(sql, "{{backtick}}", "`"))
}
func (m *Migration) ensureImportedEntryPageTable(ctx context.Context) error {
	sql := `CREATE TABLE IF NOT EXISTS zenstats_events_db.imported_entry_pages
(
    {{backtick}}site_id{{backtick}} UInt64,
    {{backtick}}date{{backtick}} Date,
    {{backtick}}entry_page{{backtick}} String,
    {{backtick}}visitors{{backtick}} UInt64,
    {{backtick}}entrances{{backtick}} UInt64,
    {{backtick}}visit_duration{{backtick}} UInt64,
    {{backtick}}bounces{{backtick}} UInt32,
    {{backtick}}import_id{{backtick}} UInt64,
    {{backtick}}pageviews{{backtick}} UInt64
)
ENGINE = MergeTree
ORDER BY (site_id, date, entry_page)
SETTINGS index_granularity = 8192, replicated_deduplication_window = 0;`

	return m.conn.Exec(ctx, strings.ReplaceAll(sql, "{{backtick}}", "`"))
}

func (m *Migration) ensureImportedDevicesTable(ctx context.Context) error {
	sql := `CREATE TABLE IF NOT EXISTS zenstats_events_db.imported_devices
(
    {{backtick}}site_id{{backtick}} UInt64,
    {{backtick}}date{{backtick}} Date,
    {{backtick}}device{{backtick}} String,
    {{backtick}}visitors{{backtick}} UInt64,
    {{backtick}}visits{{backtick}} UInt64,
    {{backtick}}visit_duration{{backtick}} UInt64,
    {{backtick}}bounces{{backtick}} UInt32,
    {{backtick}}import_id{{backtick}} UInt64,
    {{backtick}}pageviews{{backtick}} UInt64
)
ENGINE = MergeTree
ORDER BY (site_id, date, device)
SETTINGS index_granularity = 8192, replicated_deduplication_window = 0;`

	return m.conn.Exec(ctx, strings.ReplaceAll(sql, "{{backtick}}", "`"))
}

func (m *Migration) ensureImportedCustomEventsTable(ctx context.Context) error {
	sql := `CREATE TABLE IF NOT EXISTS zenstats_events_db.imported_custom_events
(
    {{backtick}}site_id{{backtick}} UInt64,
    {{backtick}}import_id{{backtick}} UInt64,
    {{backtick}}date{{backtick}} Date,
    {{backtick}}name{{backtick}} String CODEC(ZSTD(3)),
    {{backtick}}link_url{{backtick}} String CODEC(ZSTD(3)),
    {{backtick}}path{{backtick}} String CODEC(ZSTD(3)),
    {{backtick}}visitors{{backtick}} UInt64,
    {{backtick}}events{{backtick}} UInt64
)
ENGINE = MergeTree
ORDER BY (site_id, import_id, date, name)
SETTINGS replicated_deduplication_window = 0, index_granularity = 8192;`

	return m.conn.Exec(ctx, strings.ReplaceAll(sql, "{{backtick}}", "`"))
}
func (m *Migration) ensureImportedBrowsersTable(ctx context.Context) error {
	sql := `CREATE TABLE IF NOT EXISTS zenstats_events_db.imported_browsers
(
    {{backtick}}site_id{{backtick}} UInt64,
    {{backtick}}date{{backtick}} Date,
    {{backtick}}browser{{backtick}} String,
    {{backtick}}visitors{{backtick}} UInt64,
    {{backtick}}visits{{backtick}} UInt64,
    {{backtick}}visit_duration{{backtick}} UInt64,
    {{backtick}}bounces{{backtick}} UInt32,
    {{backtick}}import_id{{backtick}} UInt64,
    {{backtick}}pageviews{{backtick}} UInt64,
    {{backtick}}browser_version{{backtick}} String
)
ENGINE = MergeTree
ORDER BY (site_id, date, browser)
SETTINGS index_granularity = 8192, replicated_deduplication_window = 0;`

	return m.conn.Exec(ctx, strings.ReplaceAll(sql, "{{backtick}}", "`"))
}

func (m *Migration) ensureSessionsTable(ctx context.Context) error {
	sql := `CREATE TABLE IF NOT EXISTS zenstats_events_db.sessions
(
    {{backtick}}start{{backtick}} DateTime CODEC(Delta(4), LZ4),
    {{backtick}}timestamp{{backtick}} DateTime CODEC(Delta(4), LZ4),
    {{backtick}}session_id{{backtick}} UInt64,
    {{backtick}}version{{backtick}} UInt64,
    {{backtick}}sign{{backtick}} Int8,
    {{backtick}}is_bounce{{backtick}} UInt8,
    {{backtick}}entry_page{{backtick}} String CODEC(ZSTD(3)),
    {{backtick}}exit_page{{backtick}} String CODEC(ZSTD(3)),
    {{backtick}}pageviews{{backtick}} Int32,
    {{backtick}}events{{backtick}} Int32,
    {{backtick}}duration{{backtick}} UInt32,

    {{backtick}}site_id{{backtick}} UInt64,
    {{backtick}}user_id{{backtick}} UInt64,
    {{backtick}}url{{backtick}} String CODEC(ZSTD(3)),
    {{backtick}}hostname{{backtick}} String CODEC(ZSTD(3)),
    {{backtick}}pathname{{backtick}} String CODEC(ZSTD(3)),
    {{backtick}}referrer{{backtick}} String CODEC(ZSTD(3)),
    {{backtick}}referrer_source{{backtick}} String CODEC(ZSTD(3)),
    {{backtick}}operating_system{{backtick}} LowCardinality(String),
    {{backtick}}screen_size{{backtick}} LowCardinality(String),
    {{backtick}}utm_medium{{backtick}} String CODEC(ZSTD(3)),
    {{backtick}}utm_source{{backtick}} String CODEC(ZSTD(3)),
    {{backtick}}utm_content{{backtick}} String CODEC(ZSTD(3)),
    {{backtick}}utm_term{{backtick}} String CODEC(ZSTD(3)),
    {{backtick}}utm_campaign{{backtick}} String CODEC(ZSTD(3)),
    {{backtick}}entry_meta.key{{backtick}} Array(String) CODEC(ZSTD(3)),
    {{backtick}}entry_meta.value{{backtick}} Array(String) CODEC(ZSTD(3)),
    {{backtick}}browser{{backtick}} LowCardinality(String),
    {{backtick}}browser_version{{backtick}} LowCardinality(String),
    {{backtick}}user_agent{{backtick}} String CODEC(ZSTD(3)),
    {{backtick}}operating_system_version{{backtick}} LowCardinality(String),

    {{backtick}}ipv4{{backtick}} IPv4,
    {{backtick}}country_code{{backtick}} FixedString(2),
    {{backtick}}continent_geoname_id{{backtick}} String,
    {{backtick}}city_geoname_id{{backtick}} String,
    {{backtick}}coordinates{{backtick}} Tuple(Float64, Float64),

    {{backtick}}ipv6{{backtick}} IPv6,
    {{backtick}}channel{{backtick}} LowCardinality(String),

    {{backtick}}city{{backtick}} String ALIAS city_geoname_id,
    {{backtick}}continent{{backtick}} String ALIAS continent_geoname_id,
    {{backtick}}country{{backtick}} LowCardinality(FixedString(2)) ALIAS country_code,
    {{backtick}}entry_page_hostname{{backtick}} String ALIAS hostname,
    {{backtick}}os{{backtick}} LowCardinality(String) ALIAS operating_system,
    {{backtick}}os_version{{backtick}} LowCardinality(String) ALIAS operating_system_version,
    {{backtick}}device{{backtick}} LowCardinality(String) ALIAS screen_size,
    {{backtick}}source{{backtick}} String ALIAS referrer_source,
    {{backtick}}country_name{{backtick}} String ALIAS dictGet('zenstats_events_db.location_data_dict', 'name', ('country', country_code)),
    {{backtick}}city_name{{backtick}} String ALIAS dictGet('zenstats_events_db.location_data_dict', 'name', ('city', city_geoname_id)),
    {{backtick}}continent_name{{backtick}} String ALIAS dictGet('zenstats_events_db.location_data_dict', 'name', ('continent', continent_geoname_id)),
    INDEX minmax_timestamp timestamp TYPE minmax GRANULARITY 1
)
ENGINE = VersionedCollapsingMergeTree(sign, version)
PARTITION BY toYYYYMM(start)
PRIMARY KEY (site_id, toDate(start), user_id, session_id)
ORDER BY (site_id, toDate(start), user_id, session_id)
SAMPLE BY user_id
SETTINGS index_granularity = 8192;`

	return m.conn.Exec(ctx, strings.ReplaceAll(sql, "{{backtick}}", "`"))
}

func (m *Migration) ensureEventsTable(ctx context.Context) error {
	sql := `CREATE TABLE IF NOT EXISTS zenstats_events_db.events
(
    {{backtick}}timestamp{{backtick}} DateTime CODEC(Delta(4), LZ4),
    {{backtick}}name{{backtick}} LowCardinality(String) COMMENT 'Event name',
    {{backtick}}site_id{{backtick}} UInt64,
    {{backtick}}user_id{{backtick}} UInt64,
    {{backtick}}session_id{{backtick}} UInt64,
    {{backtick}}url{{backtick}} String CODEC(ZSTD(3)),
    {{backtick}}hostname{{backtick}} String CODEC(ZSTD(3)),
    {{backtick}}pathname{{backtick}} String CODEC(ZSTD(3)),
    {{backtick}}referrer{{backtick}} String CODEC(ZSTD(3)),
    {{backtick}}referrer_source{{backtick}} String CODEC(ZSTD(3)),
    {{backtick}}operating_system{{backtick}} LowCardinality(String),
    {{backtick}}screen_size{{backtick}} LowCardinality(String),
    {{backtick}}utm_medium{{backtick}} String CODEC(ZSTD(3)),
    {{backtick}}utm_source{{backtick}} String CODEC(ZSTD(3)),
    {{backtick}}utm_content{{backtick}} String CODEC(ZSTD(3)),
    {{backtick}}utm_term{{backtick}} String CODEC(ZSTD(3)),
    {{backtick}}utm_campaign{{backtick}} String CODEC(ZSTD(3)),
    {{backtick}}meta.key{{backtick}} Array(String) CODEC(ZSTD(3)),
    {{backtick}}meta.value{{backtick}} Array(String) CODEC(ZSTD(3)),
    {{backtick}}browser{{backtick}} LowCardinality(String),
    {{backtick}}browser_version{{backtick}} LowCardinality(String),
    {{backtick}}user_agent{{backtick}} String CODEC(ZSTD(3)),
    {{backtick}}operating_system_version{{backtick}} LowCardinality(String),
    {{backtick}}engagement_time{{backtick}} UInt32,
    {{backtick}}scroll_depth{{backtick}} UInt8,

    {{backtick}}ipv4{{backtick}} IPv4,
    {{backtick}}country_code{{backtick}} FixedString(2), --IsoCode
    {{backtick}}continent_geoname_id{{backtick}} String,
    {{backtick}}city_geoname_id{{backtick}} String,
    {{backtick}}coordinates{{backtick}} Tuple(Float64, Float64),

    {{backtick}}ipv6{{backtick}} IPv6,

    {{backtick}}city{{backtick}} String ALIAS city_geoname_id,
    {{backtick}}continent{{backtick}} String ALIAS continent_geoname_id,
    {{backtick}}country{{backtick}} LowCardinality(FixedString(2)) ALIAS country_code,
    {{backtick}}os{{backtick}} LowCardinality(String) ALIAS operating_system,
    {{backtick}}os_version{{backtick}} LowCardinality(String) ALIAS operating_system_version,
    {{backtick}}device{{backtick}} LowCardinality(String) ALIAS screen_size,
    {{backtick}}source{{backtick}} String ALIAS referrer_source,
    {{backtick}}country_name{{backtick}} String ALIAS dictGet('zenstats_events_db.location_data_dict', 'name', ('country', country_code)),
    {{backtick}}city_name{{backtick}} String ALIAS dictGet('zenstats_events_db.location_data_dict', 'name', ('city', city_geoname_id)),
    {{backtick}}continent_name{{backtick}} String ALIAS dictGet('zenstats_events_db.location_data_dict', 'name', ('continent', continent_geoname_id)),
    {{backtick}}channel{{backtick}} LowCardinality(String)
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(timestamp)
PRIMARY KEY (site_id, toDate(timestamp), name, user_id)
ORDER BY (site_id, toDate(timestamp), name, user_id, timestamp)
SAMPLE BY user_id
SETTINGS index_granularity = 8192;`

	return m.conn.Exec(ctx, strings.ReplaceAll(sql, "{{backtick}}", "`"))
}
