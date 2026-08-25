package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/config"
	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/emailverificationtoken"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/passwordresettoken"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/user"
	"github.com/zenstats/zenstats/pkg/globals"
	"gopkg.in/gomail.v2"
)

type EmailService struct {
	db *postgresql.Client
}

var GetEmailService = sync.OnceValue(func() *EmailService {
	db := globals.GetDB()
	if db == nil {
		panic("DB is not initialized")
	}
	return &EmailService{db: db}
})

// SendVerificationEmail 发送验证邮件
func (s *EmailService) SendVerificationEmail(ctx context.Context, userID int64, email string, baseURL string) error {
	// 如果没有提供 email，从数据库获取
	if email == "" {
		u, err := s.db.Client.User.Get(ctx, userID)
		if err != nil {
			return fmt.Errorf("failed to get user: %w", err)
		}
		email = u.Email
	}

	// 生成验证 token
	token, err := generateToken()
	if err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}

	// 存储验证信息到数据库（有效期 24 小时）
	_, err = s.db.Client.EmailVerificationToken.Create().
		SetUserID(userID).
		SetToken(token).
		SetEmail(email).
		SetExpiresAt(time.Now().Add(24 * time.Hour)).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to save verification token: %w", err)
	}

	// 构建验证链接
	if baseURL == "" {
		baseURL = "http://localhost"
	}
	verifyURL := fmt.Sprintf("%s/verify-email?token=%s", baseURL, token)

	// 发送邮件
	return sendEmail(email, "验证您的邮箱", fmt.Sprintf(`
		<h2>验证您的邮箱</h2>
		<p>请点击以下链接验证您的邮箱：</p>
		<p><a href="%s">%s</a></p>
		<p>此链接将在24小时后失效。</p>
		<p>如果您没有注册账户，请忽略此邮件。</p>
	`, verifyURL, verifyURL), EmailTypeVerification, nil)
}

// ErrBaseURLNotConfigured is returned when an absolute link must be generated but
// base_url (ZENSTATS_BASE_URL / ZENSTATS_DOMAIN) is not configured.
var ErrBaseURLNotConfigured = errors.New("base_url is not configured; set ZENSTATS_BASE_URL (or ZENSTATS_DOMAIN)")

// GetBaseURLFromRequest 返回用于拼接邮件链接的基础 URL。
//
// 安全：只信任配置项 base_url，绝不回退到 Origin / Referer / Host 请求头——
// 这些头由客户端完全控制，攻击者可借此把密码重置 / 邮箱验证链接指向自己的域名
// 并窃取 token（reset poisoning）。未配置时返回 ErrBaseURLNotConfigured，由调用方决定跳过发送。
func GetBaseURLFromRequest(_ *gin.Context) (string, error) {
	base := strings.TrimRight(config.Conf.BaseURL, "/")
	if base == "" {
		return "", ErrBaseURLNotConfigured
	}
	return base, nil
}

// VerifyEmail 验证邮箱
func (s *EmailService) VerifyEmail(ctx context.Context, token string) error {
	// 从数据库查找验证信息
	verification, err := s.db.Client.EmailVerificationToken.Query().
		Where(emailverificationtoken.Token(token)).
		Only(ctx)
	if err != nil {
		return errors.New("invalid or expired verification token")
	}

	// 检查是否过期
	if time.Now().After(verification.ExpiresAt) {
		// 删除过期的 token
		_ = s.db.Client.EmailVerificationToken.DeleteOne(verification).Exec(ctx)
		return errors.New("verification token has expired")
	}

	// 如果用户已经验证过邮箱，直接返回成功（重复点击链接的场景）
	u, err := s.db.Client.User.Get(ctx, verification.UserID)
	if err == nil && u.EmailVerified {
		return nil
	}

	// 更新用户邮箱验证状态
	_, err = s.db.Client.User.Update().
		Where(user.ID(verification.UserID)).
		SetEmailVerified(true).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

// ResendVerificationEmail 重新发送验证邮件
func (s *EmailService) ResendVerificationEmail(ctx context.Context, userID int64, baseURL string) error {
	// 获取用户信息
	u, err := s.db.Client.User.Get(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if u.EmailVerified {
		return errors.New("email already verified")
	}

	return s.SendVerificationEmail(ctx, userID, u.Email, baseURL)
}

// IsEmailVerified 检查邮箱是否已验证
func (s *EmailService) IsEmailVerified(ctx context.Context, userID int64) (bool, error) {
	u, err := s.db.Client.User.Get(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("failed to get user: %w", err)
	}
	return u.EmailVerified, nil
}

// IsAdmin 检查用户是否是管理员
func (s *EmailService) IsAdmin(ctx context.Context, userID int64) (bool, error) {
	u, err := s.db.Client.User.Get(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("failed to get user: %w", err)
	}
	return u.IsAdmin, nil
}

// SendPasswordResetEmail 发送密码重置邮件
func (s *EmailService) SendPasswordResetEmail(ctx context.Context, userID int64, email string, baseURL string) error {
	// 生成重置 token
	token, err := generateToken()
	if err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}

	// 存储重置 token（有效期 1 小时）
	_, err = s.db.Client.PasswordResetToken.Create().
		SetUserID(userID).
		SetToken(token).
		SetEmail(email).
		SetExpiresAt(time.Now().Add(1 * time.Hour)).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to save reset token: %w", err)
	}

	// 构建重置链接
	if baseURL == "" {
		baseURL = "http://localhost"
	}
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", baseURL, token)

	// 发送邮件
	return sendEmail(email, "重置密码", fmt.Sprintf(`
		<h2>重置密码</h2>
		<p>请点击以下链接重置您的密码：</p>
		<p><a href="%s">%s</a></p>
		<p>此链接将在1小时后失效。</p>
		<p>如果您没有请求重置密码，请忽略此邮件。</p>
	`, resetURL, resetURL), EmailTypePasswordReset, nil)
}

// VerifyPasswordResetToken 验证密码重置 token
func (s *EmailService) VerifyPasswordResetToken(ctx context.Context, token string) (*ent.PasswordResetToken, error) {
	resetToken, err := s.db.Client.PasswordResetToken.Query().
		Where(
			passwordresettoken.Token(token),
			passwordresettoken.Used(false),
		).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired reset token")
	}

	// 检查是否过期
	if time.Now().After(resetToken.ExpiresAt) {
		return nil, fmt.Errorf("reset token has expired")
	}

	return resetToken, nil
}

// MarkPasswordResetTokenUsed 标记密码重置 token 已使用
func (s *EmailService) MarkPasswordResetTokenUsed(ctx context.Context, tokenID int64) error {
	return s.db.Client.PasswordResetToken.UpdateOneID(tokenID).
		SetUsed(true).
		Exec(ctx)
}

// generateToken 生成随机 token
func generateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// ErrSMTPNotConfigured 表示未配置 SMTP Host，定时任务类邮件（报告/告警）应跳过发送。
var ErrSMTPNotConfigured = errors.New("smtp not configured")

// smtpConfigured 判断是否已配置 SMTP Host（所有发信路径共用的唯一判定）。
func smtpConfigured() bool {
	return config.Conf != nil && config.Conf.SMTP.Host != ""
}

// sendEmail 发送邮件，并记录发送结果到 EmailLog。
// 未配置 SMTP 时记录 skipped 并返回 nil（认证类邮件不阻断业务流程）。
func sendEmail(to, subject, body, emailType string, siteID *int64) error {
	if !smtpConfigured() {
		// 如果没有配置 SMTP，只记录日志不发送
		slog.Debug("smtp not configured, skipping email", "to", to, "subject", subject, "type", emailType)
		GetEmailLogService().Record(context.Background(), to, subject, emailType, EmailStatusSkipped, "SMTP not configured", siteID)
		return nil
	}

	smtp := config.Conf.SMTP
	from := smtp.From
	if from == "" {
		from = "noreply@zenstats.io"
	}
	port := smtp.Port
	if port == 0 {
		port = 587
	}

	m := gomail.NewMessage()
	m.SetHeader("From", from)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)

	d := gomail.NewDialer(smtp.Host, port, smtp.Username, smtp.Password)
	err := d.DialAndSend(m)
	GetEmailLogService().RecordSend(context.Background(), to, subject, emailType, siteID, err)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}
