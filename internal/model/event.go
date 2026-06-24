package model

import (
	"encoding/json"
	"time"
)

// EventRequest 前端 SDK 上报的事件数据结构，字段使用短 key 以减少传输体积。
type EventRequest struct {
	Timestamp      time.Time      `json:"t" example:"2025-01-01T00:00:00Z"`
	Hash           int8           `form:"h" json:"h" example:"0"`
	EventName      string         `form:"n" json:"n" example:"pageview"`
	JSVersion      string         `form:"v" json:"v" example:"1.0.0"`
	URL            string         `form:"u" json:"u" example:"https://example.com/page"`
	Domain         string         `form:"d" json:"d" example:"example.com"`
	Referrer       string         `form:"r" json:"r" example:"https://google.com"`
	Props          map[string]any `form:"p" json:"p"`
	EngagementTime int            `form:"e" json:"e" example:"15000"`
	ScrollDepth    uint8          `form:"sd" json:"sd" example:"85"`
	Interactive    bool           `form:"i" json:"i" example:"true"`
	UserAgent      string         `json:"-"`
	Ip             string         `json:"-"`
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
	Events         []TempEventRequest `json:"e"`          // batch 子事件
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
