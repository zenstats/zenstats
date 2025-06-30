package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func JSON(c *gin.Context, code int, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    code,
		Message: getStatusMessage(code),
		Data:    data,
	})
}

func Success(c *gin.Context, data interface{}) {
	JSON(c, http.StatusOK, data)
}

func Error(c *gin.Context, code int, err error) {
	c.JSON(http.StatusOK, Response{
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
