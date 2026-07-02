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
	Short: "Manually trigger traffic anomaly detection (for testing)",
	Long: `Check the specified site for traffic anomalies and send alerts if the threshold is exceeded.

示例:
  zenstats send-alert --site=example.com
  zenstats send-alert --site=example.com --email=admin@example.com`,
	Run: func(cmd *cobra.Command, args []string) {
		Init()

		if alertSite == "" {
			fmt.Fprintln(os.Stderr, "Please specify a site domain with --site")
			os.Exit(1)
		}

		fmt.Printf("检测站点 %s 的流量异常...\n", alertSite)
		if alertEmail != "" {
			fmt.Printf("Alert will be sent to: %s\n", alertEmail)
		}

		result, err := service.TestAlert(alertSite, alertEmail)
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误: %v\n", err)
			os.Exit(1)
		}

		fmt.Println(result)
		if alertEmail != "" {
			fmt.Println("✅ Alert email sent")
		} else {
			fmt.Println("💡 Use --email flag to send the alert")
		}
	},
}

func init() {
	SendAlertCmd.Flags().StringVar(&alertSite, "site", "", "站点域名（必填）")
	SendAlertCmd.Flags().StringVar(&alertEmail, "email", "", "Alert recipient email (optional, preview only if not set)")
	SendAlertCmd.MarkFlagRequired("site")

	RootCmd.AddCommand(SendAlertCmd)
}
