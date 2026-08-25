package postgresql

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"

	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/lib/pq" // register "postgres" driver for ent
	"github.com/zenstats/zenstats/config"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
)

type Client struct {
	Client *ent.Client
	// DB is the underlying connection pool shared with ent; used for raw statements
	// that ent does not model (advisory locks).
	DB *sql.DB
}

func NewClient() *Client {
	host := config.Conf.Database.Host
	user := config.Conf.Database.Username
	password := config.Conf.Database.Password
	port := config.Conf.Database.Port
	dbname := config.Conf.Database.Database

	dsn := fmt.Sprintf(
		"postgresql://%s:%s@%s:%d/%s?sslmode=disable",
		user,
		password,
		host,
		port,
		dbname,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		slog.Error("failed to open postgresql database", "error", err)
		os.Exit(1)
	}
	client := ent.NewClient(ent.Driver(entsql.OpenDB("postgres", db)))

	return &Client{Client: client, DB: db}
}
