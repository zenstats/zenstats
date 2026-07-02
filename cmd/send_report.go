package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/zenstats/zenstats/internal/service"
)

var (
	sendReportSite   string
	sendReportPeriod string
	sendReportEmail  string
)

var SendReportCmd = &cobra.Command{
	Use:   "send-report",
	Short: "手动发送一份邮件报告（测试用）",
	Long: `Send a weekly or monthly report to a specified email for testing.

示例:
  zenstats send-report --site=example.com --period=weekly --email=admin@example.com
  zenstats send-report --site=example.com --period=monthly`,
	Run: func(cmd *cobra.Command, args []string) {
		Init()

		ctx := context.Background()

		if sendReportPeriod != "weekly" && sendReportPeriod != "monthly" {
			fmt.Fprintf(os.Stderr, "无效的 period: %s (可选: weekly, monthly)\n", sendReportPeriod)
			os.Exit(1)
		}

		siteSvc := service.GetSiteService()
		siteEnt, err := siteSvc.GetSiteByDomain(ctx, sendReportSite)
		if err != nil {
			fmt.Fprintf(os.Stderr, "站点未找到: %s (%v)\n", sendReportSite, err)
			os.Exit(1)
		}

		recipient := sendReportEmail
		if recipient == "" {
			ownerID, err := siteSvc.GetSiteOwnerUserID(ctx, siteEnt.ID)
			if err != nil || ownerID == 0 {
				fmt.Fprintf(os.Stderr, "无法找到站点所有者，请用 --email 指定收件人\n")
				os.Exit(1)
			}
			userSvc := service.GetUserService()
			usr, err := userSvc.GetUserByID(ctx, ownerID)
			if err != nil || usr.Email == "" {
				fmt.Fprintf(os.Stderr, "无法找到站点所有者邮箱，请用 --email 指定收件人\n")
				os.Exit(1)
			}
			recipient = usr.Email
		}

		html := service.BuildTestReportHTML(sendReportSite, sendReportPeriod)

		name := "Weekly"
		if sendReportPeriod == "monthly" {
			name = "Monthly"
		}
		subject := fmt.Sprintf("Zenstats %s report for %s (test)", name, sendReportSite)

		fmt.Printf("收件人: %s\n", recipient)
		fmt.Printf("主题: %s\n", subject)
		fmt.Println("--- HTML 预览 ---")
		fmt.Println(html)
		fmt.Println("--- 发送中 ---")

		if err := service.SendTestReportMail(recipient, subject, html); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to send: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("✅ 测试报告已发送")
	},
}

func init() {
	SendReportCmd.Flags().StringVar(&sendReportSite, "site", "", "Site domain (required)")
	SendReportCmd.Flags().StringVar(&sendReportPeriod, "period", "weekly", "Report period: weekly or monthly")
	SendReportCmd.Flags().StringVar(&sendReportEmail, "email", "", "Recipient email(optional, defaults to site owner)")
	SendReportCmd.MarkFlagRequired("site")

	RootCmd.AddCommand(SendReportCmd)
}
