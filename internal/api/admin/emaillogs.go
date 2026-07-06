package admin

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/pkg/response"
)

// EmailLogHandler 邮件记录管理处理器。
type EmailLogHandler struct {
	svc *service.EmailLogService
}

// NewEmailLogHandler 创建邮件记录处理器实例。
func NewEmailLogHandler() *EmailLogHandler {
	return &EmailLogHandler{svc: service.GetEmailLogService()}
}

// EmailLogListResponse 邮件记录分页响应。
type EmailLogListResponse struct {
	Logs       []*service.EmailLog `json:"logs"`
	TotalCount int                 `json:"total_count"`
	Page       int                 `json:"page"`
	PageSize   int                 `json:"page_size"`
}

// List 获取邮件发送记录（分页，支持按状态/类型筛选）。
//
//	@Summary		获取邮件发送记录
//	@Description	列出系统发送的所有邮件及其发送状态，支持分页和按状态/类型筛选。
//	@Tags			管理员
//	@Security		BearerAuth
//	@Produce		json
//	@Param			page		query		int		false	"页码"		default(1)
//	@Param			page_size	query		int		false	"每页数量"	default(20)
//	@Param			status		query		string	false	"发送状态筛选：sent / failed / skipped"
//	@Param			type		query		string	false	"邮件类型筛选：verification / password_reset / email_report / traffic_alert"
//	@Success		200			{object}	response.SuccessResponse{data=EmailLogListResponse}
//	@Failure		401			{object}	response.ErrorResponse
//	@Failure		500			{object}	response.ErrorResponse
//	@Router			/admin/email-logs [get]
func (h *EmailLogHandler) List() gin.HandlerFunc {
	return func(c *gin.Context) {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
		if page < 1 {
			page = 1
		}
		if pageSize < 1 || pageSize > 100 {
			pageSize = 20
		}
		status := c.Query("status")
		emailType := c.Query("type")
		offset := (page - 1) * pageSize

		logs, err := h.svc.List(c.Request.Context(), offset, pageSize, status, emailType)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}
		total, err := h.svc.Count(c.Request.Context(), status, emailType)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err)
			return
		}

		response.Success(c, &EmailLogListResponse{
			Logs:       logs,
			TotalCount: total,
			Page:       page,
			PageSize:   pageSize,
		})
	}
}
