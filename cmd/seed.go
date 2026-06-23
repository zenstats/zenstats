package cmd

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/spf13/cobra"
	"github.com/zenstats/zenstats/internal/event"
	"github.com/zenstats/zenstats/internal/service/seed"
	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/pkg/globals"
)

var (
	seedDays  int
	seedClean bool
	seedTest  bool
)

var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "生成测试数据，通过事件管道写入 ClickHouse",
	Long: `生成仿真的多维度测试数据。

测试模式 (--test):
  使用固定随机种子生成确定性数据，适合集成测试验证。
  数据规格: 3 天、每天 30 个会话、每会话 1-3 个 pageview
  预期产出约 150-200 个 pageview 事件、30-60 个自定义事件`,
	Run: func(cmd *cobra.Command, args []string) {
		Init()
		if seedTest {
			seedDays = 3
			rand.Seed(42)
		} else {
			rand.Seed(time.Now().UnixNano())
		}

		queue := globals.GetQueue()
		if queue == nil {
			fmt.Println("错误: 队列未初始化")
			return
		}

		eventWork, err := event.NewEventWork(queue, 1024, 24*time.Hour)
		if err != nil {
			fmt.Printf("创建事件处理器失败: %v\n", err)
			return
		}
		eventWork.Run()

		gen := seed.NewGenerator(queue, eventWork, postgresql.NewClient())
		if err := gen.Run(context.Background(), seed.RunOptions{
			Days:  seedDays,
			Clean: seedClean,
			Test:  seedTest,
		}); err != nil {
			fmt.Printf("生成数据失败: %v\n", err)
		}
	},
}

func init() {
	seedCmd.Flags().IntVarP(&seedDays, "days", "d", 30, "生成多少天的历史数据")
	seedCmd.Flags().BoolVarP(&seedClean, "clean", "c", false, "生成前清空已有数据")
	seedCmd.Flags().BoolVar(&seedTest, "test", false, "测试模式：固定随机种子，生成确定性小数据集")
	RootCmd.AddCommand(seedCmd)
}
