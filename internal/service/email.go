package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
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

var (
	emailServiceInstance *EmailService
	emailOnce            sync.Once
)

type EmailService struct {
	db *postgresql.Client
}

func GetEmailService() *EmailService {
	emailOnce.Do(func() {
		db := globals.GetDB()
		if db == nil {
			panic("DB is not initialized")
		}
		emailServiceInstance = &EmailService{db: db}
	})
	return emailServiceInstance
}

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
	`, verifyURL, verifyURL))
}

// GetBaseURLFromRequest 从请求中获取基础 URL
// 优先级：config.Conf.BaseURL > Origin > Referer > http://localhost
func GetBaseURLFromRequest(c *gin.Context) string {
	// 优先使用配置的 base_url
	if config.Conf.BaseURL != "" {
		return config.Conf.BaseURL
	}

	// 从 Origin 头获取
	if origin := c.GetHeader("Origin"); origin != "" {
		return origin
	}

	// 从 Referer 头获取
	if referer := c.GetHeader("Referer"); referer != "" {
		// 只取到 path 之前的部分
		for i := len(referer) - 1; i >= 0; i-- {
			if referer[i] == '/' && i > 8 { // 至少保留 "http://"
				return referer[:i]
			}
		}
		return referer
	}

	// 回退
	scheme := "http"
	if config.Conf.Scheme.Address != "" && config.Conf.Scheme.Address != "0.0.0.0" {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, config.Conf.Scheme.Address)
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

	// 更新用户邮箱验证状态
	_, err = s.db.Client.User.Update().
		Where(user.ID(verification.UserID)).
		SetEmailVerified(true).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	// 删除已使用的验证信息
	_ = s.db.Client.EmailVerificationToken.DeleteOne(verification).Exec(ctx)

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
	`, resetURL, resetURL))
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

// getBaseURL 获取基础 URL（已弃用，请使用 GetBaseURLFromRequest）
func getBaseURL() string {
	return "http://localhost"
}

// sendEmail 发送邮件
func sendEmail(to, subject, body string) error {
	smtp := config.Conf.SMTP
	if smtp.Host == "" {
		// 如果没有配置 SMTP，只记录日志不发送
		fmt.Printf("[Email] To: %s, Subject: %s\n", to, subject)
		return nil
	}

	m := gomail.NewMessage()
	m.SetHeader("From", smtp.From)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)

	d := gomail.NewDialer(smtp.Host, smtp.Port, smtp.Username, smtp.Password)
	if err := d.DialAndSend(m); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}
