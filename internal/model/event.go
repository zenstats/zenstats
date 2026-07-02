package model

import (
	"encoding/json"
	"time"
)

// EventRequest Event data structure reported by the tracker SDK. Fields use short keys to reduce transmission size.
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
	Hash           int8             `form:"h" json:"h"`   // Hash
	EventName      string           `form:"n" json:"n"`   // Event name
	JSVersion      string           `form:"v" json:"v"`   // JS version
	URL            string           `form:"u" json:"u"`   // URL
	Domain         string           `form:"d" json:"d"`   // Domain
	Referrer       string           `form:"r" json:"r"`   // Referrer
	Props          map[string]any   `form:"p" json:"p"`   // Properties
	EngagementTime int              `form:"e" json:"e"`       // Page view time
	ScrollDepth    uint8            `form:"sd" json:"sd"`     // Scroll depth
	Interactive    *json.RawMessage `form:"i" json:"i"`       // Whether the event is interactive
	UserAgent      string
	Ip             string
}

// BatchEventRequest Batch event structure reported by the tracker SDK.
// 顶层 payload 形如 {"n":"batch","e":[<子事件>...],"v":...}，
// where "e" is an array of sub-events (different from single-event "e" = engagement time, hence a separate struct).
type BatchEventRequest struct {
	EventName string             `json:"n"` // Always "batch"
	JSVersion string             `json:"v"` // JS version
	Domain    string             `json:"d"` // batch 级别Domain（子事件可覆盖）
	Events    []TempEventRequest `json:"e"` // Array of sub-events
}

// {
//     "n": "pageview",
//     "v": "1",
//     "u": "example.com/zenstats/index.html",
//     "d": "example.com",
//     "r": null,
//     "p": {}
// }
