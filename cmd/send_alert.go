package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/zenstats/zenstats/internal/service"
)

var (
	alertSite  string
	alertEmail string
)

var SendAlertCmd = &cobra.Command{
	Use:   "send-alert",
	Short: "手动触发流量异常检测（测试用）",
	Long: `手动检测指定站点的流量异常，超过阈值时发送告警邮件。

示例:
  zenstats send-alert --site=example.com
  zenstats send-alert --site=example.com --email=admin@example.com`,
	Run: func(cmd *cobra.Command, args []string) {
		Init()

		if alertSite == "" {
			fmt.Fprintln(os.Stderr, "请用 --site 指定站点域名")
			os.Exit(1)
		}

		fmt.Printf("检测站点 %s 的流量异常...\n", alertSite)
		if alertEmail != "" {
			fmt.Printf("告警将发送到: %s\n", alertEmail)
		}

		result, err := service.TestAlert(alertSite, alertEmail)
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误: %v\n", err)
			os.Exit(1)
		}

		fmt.Println(result)
		if alertEmail != "" {
			fmt.Println("✅ 告警邮件已发送")
		} else {
			fmt.Println("💡 使用 --email 参数可发送告警邮件")
		}
	},
}

func init() {
	SendAlertCmd.Flags().StringVar(&alertSite, "site", "", "站点域名（必填）")
	SendAlertCmd.Flags().StringVar(&alertEmail, "email", "", "告警收件人邮箱（可选，不指定仅预览）")
	SendAlertCmd.MarkFlagRequired("site")

	RootCmd.AddCommand(SendAlertCmd)
}
