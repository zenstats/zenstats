package external

import (
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/ingestion"
	"github.com/zenstats/zenstats/internal/model"
)

func TestCollectionQualityRejectedAndFilteredRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, body, ipType                    string
		status                                int
		received, invalid, rejected, filtered uint64
	}{
		{"bad JSON", "{", "", 400, 0, 1, 0, 0},
		{"large body", strings.Repeat("x", (1<<20)+1), "", 413, 0, 1, 0, 0},
		{"wrong field type", `{"n":1}`, "", 400, 0, 1, 0, 0},
		{"empty batch", `{"n":"batch","e":[]}`, "", 400, 0, 1, 0, 0},
		{"missing fields", `{}`, "", 400, 1, 0, 1, 0},
		{"batch children", `{"n":"batch","e":[{},{},{}]}`, "", 400, 3, 0, 3, 0},
		{"datacenter batch", `{"n":"batch","e":[{},{}]}`, "dc_ip", 202, 2, 0, 0, 2},
		{"batch limit", `{"n":"batch","e":[` + strings.Repeat(`{},`, 100) + `{}]}`, "", 400, 101, 0, 101, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := ingestion.NewMonitor()
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/api/event", strings.NewReader(tc.body))
			c.Request.Header.Set("X-Zenstats-Ip-Type", tc.ipType)
			eventHandler(nil, m)(c)
			s := m.Snapshot()
			if w.Code != tc.status || s.Counters["requests"] != 1 || s.Counters["received"] != tc.received || s.Counters["invalid_requests"] != tc.invalid || s.Counters["rejected"] != tc.rejected || s.Counters["filtered"] != tc.filtered || s.Counters["accepted"] != 0 || s.Counters["persisted"] != 0 || s.Pending.Events != 0 {
				t.Fatalf("status=%d counters=%+v pending=%+v", w.Code, s.Counters, s.Pending)
			}
		})
	}
}

func TestSingleAndBatchPreserveEventFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/api/event", nil)
	c.Request.RemoteAddr = "203.0.113.1:1234"
	c.Request.Header.Set("User-Agent", "test-browser")
	timestamp := time.Now().Add(-time.Minute).UTC().Truncate(time.Second)
	payload := `{"n":"purchase","u":"https://example.com/docs","d":"example.com","p":{"plan":"pro"},"revenue":{"amount":"12.50","currency":"USD"},"i":false,"t":"` + timestamp.Format(time.RFC3339) + `","e":15000,"sd":85,"h":1,"v":"1"}`
	var single model.TempEventRequest
	var batch model.BatchEventRequest
	if err := json.Unmarshal([]byte(payload), &single); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"n":"batch","d":"example.com","e":[`+payload+`]}`), &batch); err != nil {
		t.Fatal(err)
	}
	a, b := newEventRequest(single, single.Domain, c), newEventRequest(batch.Events[0], batch.Domain, c)
	if !reflect.DeepEqual(a, b) || b.Revenue == nil || b.Revenue.Amount.String() != "12.50" || b.Interactive || !b.Timestamp.Equal(timestamp) {
		t.Fatalf("event fields differ or were lost: single=%+v batch=%+v", a, b)
	}
}

func TestOversizedBatchIsRejectedWithoutTruncation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{"n":"batch","e":[` + strings.Repeat(`{},`, 100) + `{}]}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/event", strings.NewReader(body))
	Event(nil)(c)
	if w.Code != 400 {
		t.Fatalf("oversized batch status=%d", w.Code)
	}
}
