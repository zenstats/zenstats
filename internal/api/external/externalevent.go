package external

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/common"
	"github.com/zenstats/zenstats/pkg/globals"
	"github.com/zenstats/zenstats/pkg/utils"
)

func Event() gin.HandlerFunc {
	return func(c *gin.Context) {
		slog.Debug("received event")

		var req common.EventRequest
		body, _ := c.GetRawData()
		err := json.Unmarshal(body, &req)
		if err != nil {
			c.JSON(http.StatusBadRequest, "bad")
			return
		}
		req.Timestamp = time.Now()
		req.Ip = utils.ClientIP(c.Request)
		req.UserAgent = c.Request.UserAgent()

		queue := globals.GetQueue()
		if err = queue.Enqueue(&req); err != nil {
			c.JSON(http.StatusBadRequest, "error")
			return
		}

		c.JSON(http.StatusAccepted, "ok")
	}
}
