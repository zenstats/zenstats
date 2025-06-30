package bootstrap

import (
	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/pkg/globals"
)

func InitPostgres() {

	client := postgresql.NewClient()

	globals.SetDB(client)
}
