package external

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/common"
	"github.com/zenstats/zenstats/internal/service"
	"github.com/zenstats/zenstats/pkg/globals"
	"github.com/zenstats/zenstats/pkg/utils"
)

// setEventRequestDefaults 设置 EventRequest 的默认值
func setEventRequestDefaults(req *common.EventRequest, tempReq *common.TempEventRequest, c *gin.Context) {
	req.Timestamp = time.Now()
	req.Ip = utils.ClientIP(c.Request)
	req.UserAgent = c.Request.UserAgent()
}

// parseInteractive 解析 Interactive 字段
func parseInteractive(req *common.EventRequest, tempReq *common.TempEventRequest) {
	req.Interactive = true
	if tempReq.Interactive != nil {
		var isFalse bool
		if err := json.Unmarshal(*tempReq.Interactive, &isFalse); err == nil && !isFalse {
			req.Interactive = false
		}
	}
}

func Event(siteService *service.SiteService) gin.HandlerFunc {
	return func(c *gin.Context) {
		slog.Debug("received event")

		var tempReq common.TempEventRequest
		body, err := c.GetRawData()
		if err != nil {
			slog.Error("failed to get raw data", "error", err)
			c.JSON(http.StatusBadRequest, "bad")
			return
		}
		slog.Debug("request body", "body", string(body))

		err = json.Unmarshal(body, &tempReq)
		if err != nil {
			slog.Error("failed to unmarshal request body", "error", err)
			c.JSON(http.StatusBadRequest, "bad")
			return
		}

		can, err := siteService.IsDomainInList(c, tempReq.Domain)
		if err != nil {
			slog.Error("failed to check domain", "error", err)
			c.JSON(http.StatusBadRequest, "bad")
			return
		}
		if !can {
			slog.Warn("domain not allowed", "domain", tempReq.Domain)
			c.JSON(http.StatusBadRequest, "bad")
			return
		}

		req := common.EventRequest{
			Timestamp:      tempReq.Timestamp,
			Hash:           tempReq.Hash,
			EventName:      tempReq.EventName,
			JSVersion:      tempReq.JSVersion,
			URL:            tempReq.URL,
			Domain:         tempReq.Domain,
			Referrer:       tempReq.Referrer,
			Props:          tempReq.Props,
			EngagementTime: tempReq.EngagementTime,
			ScrollDepth:    tempReq.ScrollDepth,
		}

		setEventRequestDefaults(&req, &tempReq, c)
		parseInteractive(&req, &tempReq)

		queue := globals.GetQueue()
		if err = queue.Enqueue(&req); err != nil {
			c.JSON(http.StatusBadRequest, "error")
			return
		}

		c.JSON(http.StatusAccepted, "ok")
	}
}
