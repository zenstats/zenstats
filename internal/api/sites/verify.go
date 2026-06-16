package sites

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/pkg/response"
)

// VerificationStatus 获取站点验证状态
//
//	@Summary		获取站点验证状态
//	@Description	获取指定域名的验证状态信息，包括验证令牌（仅未验证时返回）
//	@Tags			站点验证
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			domain	path		string	true	"站点域名"
//	@Success		200		{object}	response.SuccessResponse{data=types.SiteVerificationStatus}	"成功响应"
//	@Failure		400		{object}	response.ErrorResponse	"请求参数错误"
//	@Failure		401		{object}	response.ErrorResponse	"未认证"
//	@Failure		403		{object}	response.ErrorResponse	"无权限"
//	@Failure		500		{object}	response.ErrorResponse	"服务器内部错误"
//	@Router			/sites/{domain}/verification-status [get]
func (h *SitesHandler) VerificationStatus() gin.HandlerFunc {
	return func(c *gin.Context) {
		domain := c.Param("domain")
		userID := c.GetInt64("user_id")

		status, err := h.service.GetVerificationStatus(c, domain, userID)
		if err != nil {
			response.Error(c, http.StatusNotFound, err)
			return
		}

		response.Success(c, status)
	}
}

// Verify 验证站点域名所有权
//
//	@Summary		验证站点域名所有权
//	@Description	通过获取域名下的验证文件来验证所有权
//	@Tags			站点验证
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			domain	path		string	true	"站点域名"
//	@Success		200		{string}	string	"ok"
//	@Failure		400		{object}	response.ErrorResponse	"请求参数错误或验证失败"
//	@Failure		401		{object}	response.ErrorResponse	"未认证"
//	@Failure		403		{object}	response.ErrorResponse	"无权限"
//	@Failure		500		{object}	response.ErrorResponse	"服务器内部错误"
//	@Router			/sites/{domain}/verify [post]
func (h *SitesHandler) Verify() gin.HandlerFunc {
	return func(c *gin.Context) {
		domain := c.Param("domain")
		userID := c.GetInt64("user_id")

		err := h.service.VerifySite(c, domain, userID)
		if err != nil {
			response.Error(c, http.StatusBadRequest, err)
			return
		}

		response.Success(c, gin.H{"message": "site verified successfully"})
	}
}
