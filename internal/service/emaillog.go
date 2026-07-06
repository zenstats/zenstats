package service

import (
	"context"
	"sync"
	"time"

	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	entemaillog "github.com/zenstats/zenstats/internal/store/postgresql/ent/emaillog"
	"github.com/zenstats/zenstats/pkg/globals"
)

// 邮件类型常量
const (
	EmailTypeVerification  = "verification"
	EmailTypePasswordReset = "password_reset"
	EmailTypeReport        = "email_report"
	EmailTypeTrafficAlert  = "traffic_alert"
)

// 发送状态常量
const (
	EmailStatusSent    = "sent"
	EmailStatusFailed  = "failed"
	EmailStatusSkipped = "skipped"
)

// EmailLog 邮件发送记录（仅元数据）。
type EmailLog struct {
	ID        int64     `json:"id"`
	Recipient string    `json:"recipient"`
	Subject   string    `json:"subject"`
	EmailType string    `json:"email_type"`
	Status    string    `json:"status"`
	Error     string    `json:"error,omitempty"`
	SiteID    *int64    `json:"site_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// EmailLogService 邮件记录业务逻辑。
type EmailLogService struct {
	db *postgresql.Client
}

// GetEmailLogService 获取 EmailLogService 单例。
var GetEmailLogService = sync.OnceValue(func() *EmailLogService {
	db := globals.GetDB()
	if db == nil {
		panic("DB is not initialized")
	}
	return &EmailLogService{db: db}
})

func toEmailLog(e *ent.EmailLog) *EmailLog {
	return &EmailLog{
		ID:        e.ID,
		Recipient: e.Recipient,
		Subject:   e.Subject,
		EmailType: e.EmailType,
		Status:    e.Status,
		Error:     e.Error,
		SiteID:    e.SiteID,
		CreatedAt: e.CreatedAt,
	}
}

// Record 记录一封邮件的发送结果。best-effort：写入失败不影响主流程。
func (s *EmailLogService) Record(ctx context.Context, recipient, subject, emailType, status, errMsg string, siteID *int64) {
	if s == nil || s.db == nil {
		return
	}
	q := s.db.Client.EmailLog.Create().
		SetRecipient(recipient).
		SetSubject(subject).
		SetEmailType(emailType).
		SetStatus(status)
	if errMsg != "" {
		q = q.SetError(errMsg)
	}
	if siteID != nil {
		q = q.SetSiteID(*siteID)
	}
	// 使用独立 context，避免请求 context 取消影响审计写入
	_, _ = q.Save(context.WithoutCancel(ctx))
}

// RecordSend 根据发送错误自动记录状态（err==nil → sent，否则 failed）。
func (s *EmailLogService) RecordSend(ctx context.Context, recipient, subject, emailType string, siteID *int64, sendErr error) {
	status := EmailStatusSent
	errMsg := ""
	if sendErr != nil {
		status = EmailStatusFailed
		errMsg = sendErr.Error()
	}
	s.Record(ctx, recipient, subject, emailType, status, errMsg, siteID)
}

// List 分页查询邮件记录，可按状态 / 类型筛选。
func (s *EmailLogService) List(ctx context.Context, offset, limit int, status, emailType string) ([]*EmailLog, error) {
	q := s.db.Client.EmailLog.Query()
	if status != "" {
		q = q.Where(entemaillog.Status(status))
	}
	if emailType != "" {
		q = q.Where(entemaillog.EmailType(emailType))
	}
	rows, err := q.
		Order(ent.Desc(entemaillog.FieldCreatedAt)).
		Offset(offset).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*EmailLog, len(rows))
	for i, r := range rows {
		out[i] = toEmailLog(r)
	}
	return out, nil
}

// Count 统计邮件记录数（同样支持筛选）。
func (s *EmailLogService) Count(ctx context.Context, status, emailType string) (int, error) {
	q := s.db.Client.EmailLog.Query()
	if status != "" {
		q = q.Where(entemaillog.Status(status))
	}
	if emailType != "" {
		q = q.Where(entemaillog.EmailType(emailType))
	}
	return q.Count(ctx)
}
