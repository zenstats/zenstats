package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
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
	Data interface{} `json:"data"`
}

// JSON 响应
//
//	@Description	接口返回的响应信息
func JSON(c *gin.Context, code int, data interface{}) {
	c.JSON(http.StatusOK, SuccessResponse{
		Code:    code,
		Message: getStatusMessage(code),
		Data:    data,
	})
}

// Success 成功响应
//
//	@Description	接口返回的响应信息
func Success(c *gin.Context, data interface{}) {
	JSON(c, http.StatusOK, data)
}

// Error 错误响应
//
//	@Description	接口返回的响应信息
func Error(c *gin.Context, code int, err error) {
	c.JSON(http.StatusOK, ErrorResponse{
		Code:    code,
		Message: getStatusMessage(code),
		Error:   err.Error(),
	})
}

func getStatusMessage(code int) string {
	switch code {
	case http.StatusOK:
		return "success"
	case http.StatusBadRequest:
		return "bad request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not found"
	case http.StatusInternalServerError:
		return "internal server error"
	case http.StatusServiceUnavailable:
		return "service unavailable"
	default:
		return ""
	}
}
