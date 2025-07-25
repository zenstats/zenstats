package common

import (
	"encoding/json"
	"time"
)

type EventRequest struct {
	Timestamp      time.Time      `json:"t"`
	Hash           int8           `form:"h" json:"h"`   // 哈希
	EventName      string         `form:"n" json:"n"`   // 事件名称
	JSVersion      string         `form:"v" json:"v"`   // JS版本
	URL            string         `form:"u" json:"u"`   // URL
	Domain         string         `form:"d" json:"d"`   // 域名
	Referrer       string         `form:"r" json:"r"`   // 来源
	Props          map[string]any `form:"p" json:"p"`   // 属性
	EngagementTime int            `form:"e" json:"e"`   // 页面访问时间
	ScrollDepth    uint8          `form:"sd" json:"sd"` // 滚动深度
	Interactive    bool           `form:"i" json:"i"`   // 是否交互
	UserAgent      string
	Ip             string
}

type TempEventRequest struct {
	Timestamp      time.Time        `json:"t"`
	Hash           int8             `form:"h" json:"h"`   // 哈希
	EventName      string           `form:"n" json:"n"`   // 事件名称
	JSVersion      string           `form:"v" json:"v"`   // JS版本
	URL            string           `form:"u" json:"u"`   // URL
	Domain         string           `form:"d" json:"d"`   // 域名
	Referrer       string           `form:"r" json:"r"`   // 来源
	Props          map[string]any   `form:"p" json:"p"`   // 属性
	EngagementTime int              `form:"e" json:"e"`   // 页面访问时间
	ScrollDepth    uint8            `form:"sd" json:"sd"` // 滚动深度
	Interactive    *json.RawMessage `form:"i" json:"i"`   // 是否交互
	UserAgent      string
	Ip             string
}

// {
//     "n": "pageview",
//     "v": "1",
//     "u": "example.com/zenstats/index.html",
//     "d": "example.com",
//     "r": null,
//     "p": {}
// }
