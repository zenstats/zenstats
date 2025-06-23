package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/zenstats/zenstats/internal/store/postgresql"
)

var MigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Start the server at the specified address",
	Long: `Start the server at the specified address
the address is defined in config file`,
	Run: func(cmd *cobra.Command, args []string) {
		Init()

		client := postgresql.NewClient()
		fmt.Println("Migrating...")
		if err := client.Client.Schema.Create(context.Background()); err != nil {
			fmt.Printf("failed creating schema resources: %v", err)
			os.Exit(1)
		}
		fmt.Println("Migrated")
	},
}

func init() {
	RootCmd.AddCommand(MigrateCmd)
}
