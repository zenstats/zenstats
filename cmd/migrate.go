package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/searchengines"
)

var MigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "migrate database",
	Run: func(cmd *cobra.Command, args []string) {
		Init()

		client := postgresql.NewClient()
		fmt.Println("Migrating...")
		if err := client.Client.Schema.Create(context.Background()); err != nil {
			fmt.Printf("failed creating schema resources: %v", err)
			os.Exit(1)
		}

		fmt.Println("Migrated")

		// 检查 SearchEngines 表是否有数据
		count, err := client.Client.SearchEngines.Query().Count(context.Background())
		if err != nil {
			fmt.Printf("failed counting SearchEngines records: %v", err)
			os.Exit(1)
		}
		engines := client.Client.SearchEngines.Query().Where(searchengines.NameEQ("360")).AllX(context.Background())
		fmt.Printf("engines: %v\n", engines)
		if count == 0 {
			fmt.Println("No data in SearchEngines table. Inserting default data...")
			// 定义要插入的数据
			searchEnginesData := map[string]string{
				"google.com": "Google",
				"bing.com":   "Bing",
				"so.com":     "360",
				"baidu.com":  "Baidu",
				"yandex.com": "Yandex",
				"yahoo.com":  "Yahoo",
				"github.com": "Github",
			}

			// 批量插入数据
			mutations := make([]*ent.SearchEnginesCreate, 0, len(searchEnginesData))
			for domain, name := range searchEnginesData {
				mutation := client.Client.SearchEngines.Create().
					SetDomain(domain).
					SetName(name)
				mutations = append(mutations, mutation)
			}

			if _, err := client.Client.SearchEngines.CreateBulk(mutations...).Save(context.Background()); err != nil {
				fmt.Printf("failed inserting default SearchEngines data: %v", err)
				os.Exit(1)
			}
			fmt.Println("Default data inserted into SearchEngines table.")
		}
	},
}

func init() {
	RootCmd.AddCommand(MigrateCmd)
}
