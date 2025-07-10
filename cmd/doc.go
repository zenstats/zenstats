package cmd

import (
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	_ "github.com/zenstats/zenstats/docs"
)

var DocCmd = &cobra.Command{
	Use:   "doc",
	Short: "swagger doc",
	Run: func(cmd *cobra.Command, args []string) {
		gin.SetMode(gin.ReleaseMode)
		r := gin.Default()

		// 注册 Swagger UI 路由
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

		r.Run(":8081")
	},
}

func init() {
	RootCmd.AddCommand(DocCmd)
}
