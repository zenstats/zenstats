package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/zenstats/zenstats/internal/bootstrap"
	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
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

		// 插入默认用户组（subscription plans）
		insertDefaultUserGroups(client)

		// 插入默认搜索引擎
		insertDefaultSearchEngines(client)

		// 初始化系统配置
		bootstrap.InitSystemConfig()

		fmt.Println("System config initialized")
	},
}

func insertDefaultUserGroups(client *postgresql.Client) {
	// 检查 user_groups 表是否有数据
	count, err := client.Client.UserGroup.Query().Count(context.Background())
	if err != nil {
		fmt.Printf("failed counting UserGroup records: %v", err)
		os.Exit(1)
	}

	if count == 0 {
		fmt.Println("No data in UserGroups table. Inserting default data...")

		// 定义默认套餐
		defaultGroups := []struct {
			Name                string
			Description         string
			MaxSites            int
			MaxMonthlyEvents    int
			MaxAPIKeys          int
			MaxSubAccounts      int
			CustomSearchEngines bool
			IsDefault           bool
			Price               float64
		}{
			{
				Name:                "免费版",
				Description:         "适合个人博客和小型网站",
				MaxSites:            3,
				MaxMonthlyEvents:    10000,
				MaxAPIKeys:          2,
				MaxSubAccounts:      0,
				CustomSearchEngines: false,
				IsDefault:           true,
				Price:               0,
			},
			{
				Name:                "专业版",
				Description:         "适合中型网站和企业",
				MaxSites:            10,
				MaxMonthlyEvents:    100000,
				MaxAPIKeys:          10,
				MaxSubAccounts:      5,
				CustomSearchEngines: true,
				IsDefault:           false,
				Price:               29,
			},
			{
				Name:                "企业版",
				Description:         "适合大型网站和SaaS产品",
				MaxSites:            -1,
				MaxMonthlyEvents:    -1,
				MaxAPIKeys:          -1,
				MaxSubAccounts:      -1,
				CustomSearchEngines: true,
				IsDefault:           false,
				Price:               99,
			},
		}

		// 批量插入数据
		mutations := make([]*ent.UserGroupCreate, 0, len(defaultGroups))
		for _, g := range defaultGroups {
			mutation := client.Client.UserGroup.Create().
				SetName(g.Name).
				SetDescription(g.Description).
				SetMaxSites(g.MaxSites).
				SetMaxMonthlyEvents(g.MaxMonthlyEvents).
				SetMaxAPIKeys(g.MaxAPIKeys).
				SetMaxSubAccounts(g.MaxSubAccounts).
				SetCustomSearchEngines(g.CustomSearchEngines).
				SetIsDefault(g.IsDefault).
				SetPrice(g.Price)
			mutations = append(mutations, mutation)
		}

		if _, err := client.Client.UserGroup.CreateBulk(mutations...).Save(context.Background()); err != nil {
			fmt.Printf("failed inserting default UserGroups data: %v", err)
			os.Exit(1)
		}
		fmt.Println("Default data inserted into UserGroups table.")
	}
}

func insertDefaultSearchEngines(client *postgresql.Client) {
	// 检查 SearchEngines 表是否有数据
	count, err := client.Client.SearchEngines.Query().Count(context.Background())
	if err != nil {
		fmt.Printf("failed counting SearchEngines records: %v", err)
		os.Exit(1)
	}

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
}

func init() {
	RootCmd.AddCommand(MigrateCmd)
}
