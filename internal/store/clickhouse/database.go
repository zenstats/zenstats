package clickhouse

import (
	"fmt"
	"regexp"

	"github.com/zenstats/zenstats/config"
)

var databaseIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// DatabaseName validates the configured database before it is embedded in SQL.
func DatabaseName() (string, error) {
	name := config.Conf.Clickhouse.Database
	if name == "" {
		name = "zenstats_events_db"
	}
	if !databaseIdentifier.MatchString(name) {
		return "", fmt.Errorf("invalid ClickHouse database name %q", name)
	}
	return name, nil
}
