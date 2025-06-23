package postgresql

import (
	"fmt"
	"log/slog"
	"os"

	_ "github.com/lib/pq"
	"github.com/zenstats/zenstats/config"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
)

type Client struct {
	Client *ent.Client
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

	client, err := ent.Open("postgres", dsn)
	if err != nil {
		slog.Error("failed to open postgresql database", "error", err)
		os.Exit(1)
	}

	return &Client{Client: client}
}
