package response

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/pkg/i18n"
)

// ErrorResponse 错误响应结构体
//
//	@Description	接口返回的错误响应信息
type ErrorResponse struct {
	// 错误码
	Code int `json:"code"`
	// 错误信息
	Message string `json:"message"`
	// 错误详情
	Error string `json:"error,omitempty"`
}

// SuccessResponse 成功响应结构体
//
//	@Description	接口返回的成功响应信息
type SuccessResponse struct {
	// 状态码
	Code int `json:"code"`
	// 响应信息
	Message string `json:"message"`
	// 响应数据
	Data any `json:"data"`
}

// JSON 响应
//
//	@Description	接口返回的响应信息
func JSON(c *gin.Context, code int, data any) {
	locale := getLocale(c)
	c.JSON(http.StatusOK, SuccessResponse{
		Code:    code,
		Message: getStatusMessage(code, locale),
		Data:    data,
	})
}

// Success 成功响应
//
//	@Description	接口返回的响应信息
func Success(c *gin.Context, data any) {
	JSON(c, http.StatusOK, data)
}

// Error 错误响应
//
//	@Description	接口返回的响应信息
func Error(c *gin.Context, code int, err error) {
	locale := getLocale(c)
	c.JSON(http.StatusOK, ErrorResponse{
		Code:    code,
		Message: getStatusMessage(code, locale),
		Error:   err.Error(),
	})
}

// ErrorWithKey 使用 i18n key 的错误响应
func ErrorWithKey(c *gin.Context, code int, key string, args ...any) {
	locale := getLocale(c)
	msg := i18n.T(locale, key)
	if len(args) > 0 {
		msg = fmt.Sprintf(msg, args...)
	}
	c.JSON(http.StatusOK, ErrorResponse{
		Code:    code,
		Message: getStatusMessage(code, locale),
		Error:   msg,
	})
}

// ErrorWithKeyAndMessage 使用 i18n key 的错误响应，同时自定义 message 字段
func ErrorWithKeyAndMessage(c *gin.Context, code int, messageKey string, errorKey string, args ...any) {
	locale := getLocale(c)
	msg := i18n.T(locale, errorKey)
	if len(args) > 0 {
		msg = fmt.Sprintf(msg, args...)
	}
	c.JSON(http.StatusOK, ErrorResponse{
		Code:    code,
		Message: i18n.T(locale, messageKey),
		Error:   msg,
	})
}

func getStatusMessage(code int, locale string) string {
	var key string
	switch code {
	case http.StatusOK:
		key = "response.success"
	case http.StatusBadRequest:
		key = "response.bad_request"
	case http.StatusUnauthorized:
		key = "response.unauthorized"
	case http.StatusForbidden:
		key = "response.forbidden"
	case http.StatusNotFound:
		key = "response.not_found"
	case http.StatusInternalServerError:
		key = "response.internal_error"
	case http.StatusServiceUnavailable:
		key = "response.service_unavailable"
	default:
		return ""
	}
	return i18n.T(locale, key)
}

func getLocale(c *gin.Context) string {
	if locale, exists := c.Get("locale"); exists {
		return locale.(string)
	}
	return "en"
}

// Deprecated: Use ErrorWithKey instead
func Errorf(c *gin.Context, code int, format string, args ...any) {
	err := fmt.Errorf(format, args...)
	Error(c, code, err)
}

// Deprecated: Use ErrorWithKey instead
func ErrorNew(c *gin.Context, code int, err error) {
	Error(c, code, err)
}

// Ensure fmt import is used
var _ = fmt.Sprintf
