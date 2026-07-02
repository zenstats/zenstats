package model

import (
	"encoding/json"
	"testing"
)

// TestBatchEventRequestDecodesEArray 验证前端 SDK 的 batch 载荷 {n:'batch', e:[...]}
// 能正确解析出子事件数组（此前 Events 用 json:"events" 标签导致整包 400）。
func TestBatchEventRequestDecodesEArray(t *testing.T) {
	body := []byte(`{"n":"batch","v":"1.0","d":"example.com","e":[
		{"n":"pageview","u":"https://example.com/a","d":"example.com","e":1500},
		{"n":"custom","u":"https://example.com/b","d":"example.com","e":2000}
	]}`)

	var batch BatchEventRequest
	if err := json.Unmarshal(body, &batch); err != nil {
		t.Fatalf("batch unmarshal failed: %v", err)
	}
	if batch.EventName != "batch" {
		t.Fatalf("expected event name batch, got %q", batch.EventName)
	}
	if len(batch.Events) != 2 {
		t.Fatalf("expected 2 sub-events, got %d", len(batch.Events))
	}
	if batch.Events[0].EventName != "pageview" || batch.Events[0].EngagementTime != 1500 {
		t.Fatalf("sub-event 0 decoded wrong: %+v", batch.Events[0])
	}
	if batch.Events[1].EngagementTime != 2000 {
		t.Fatalf("sub-event 1 engagement wrong: %+v", batch.Events[1])
	}
}

// TestSingleEventDecodesEngagementInt Verifies that single-event "e" still parses as engagement time (int).
func TestSingleEventDecodesEngagementInt(t *testing.T) {
	body := []byte(`{"n":"engagement","u":"https://example.com/a","d":"example.com","e":3000}`)
	var req TempEventRequest
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("single event unmarshal failed: %v", err)
	}
	if req.EngagementTime != 3000 {
		t.Fatalf("expected engagement 3000, got %d", req.EngagementTime)
	}
}
