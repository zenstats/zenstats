package external

import (
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zenstats/zenstats/internal/model"
)

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
