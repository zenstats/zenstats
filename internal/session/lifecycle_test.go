package session

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/zenstats/zenstats/internal/store/clickhouse/models"
)

func testLifecycleManager() *SessionManager {
	return &SessionManager{cache: expirable.NewLRU[string, *models.Sessions](10, nil, sessionCacheTTL), writeBuffer: NewWriteBuffer(context.Background(), 1000, time.Hour)}
}

func TestSessionInactivityUsesEventTime(t *testing.T) {
	start := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	session := &models.Sessions{Start: start, Timestamp: start.Add(time.Minute)}
	for _, tc := range []struct {
		offset time.Duration
		want   bool
	}{
		{29 * time.Minute, true}, {30 * time.Minute, true}, {31 * time.Minute, false}, {45 * time.Minute, false},
	} {
		if got := sessionContainsEvent(session, session.Timestamp.Add(tc.offset)); got != tc.want {
			t.Errorf("gap=%s contains=%v", tc.offset, got)
		}
	}
	if sessionContainsEvent(session, start.Add(-time.Second)) {
		t.Fatal("late event was attached to a session starting later")
	}
	sm := testLifecycleManager()
	sm.writeBuffer.Add(&models.Sessions{SessionId: 1, SiteId: 1, UserId: 1, Sign: 1, Version: 1, Start: start, Timestamp: start})
	if got := sm.writeBuffer.FindPendingAt(1, 1, start.Add(45*time.Minute)); got != nil {
		t.Fatal("expired pending session was restored")
	}
}

func TestLateEventsPreserveSessionEndAndCache(t *testing.T) {
	sm := testLifecycleManager()
	start := time.Now().Add(-20 * time.Minute)
	old := sm.newSession(&models.Events{SiteId: 1, UserId: 1, Name: "pageview", Timestamp: start, PathName: "/entry"})
	old.Timestamp = start.Add(10 * time.Minute)
	old.Duration = 600
	old.ExitPage = "/latest"
	old.IsBounce = 0
	sm.writeBuffer.Add(old)
	late := &models.Events{SiteId: 1, UserId: 1, Name: "pageview", Timestamp: start.Add(5 * time.Minute), PathName: "/older"}
	current := sm.newSession(&models.Events{SiteId: 1, UserId: 1, Name: "pageview", Timestamp: time.Now().Add(time.Hour)})
	sm.updateSessionCache(current)
	got, err := sm.handleEvent(late, current)
	if err != nil {
		t.Fatal(err)
	}
	if got.SessionId != old.SessionId || got.Duration != 600 || got.ExitPage != "/latest" || !got.Timestamp.Equal(old.Timestamp) || got.PageViews != old.PageViews+1 {
		t.Fatalf("late event corrupted session: %+v", got)
	}
	if cached := sm.findSession(late); cached.SessionId != current.SessionId {
		t.Fatal("late event replaced current session cache")
	}
	engagement := sm.updateSession(got, &models.Events{Name: "engagement", Timestamp: start.Add(3 * time.Minute)})
	if engagement.Duration != 600 || engagement.ExitPage != "/latest" {
		t.Fatal("late engagement regressed duration or exit page")
	}
}

func TestEngagementPreservesBounceAndTombstoneVersion(t *testing.T) {
	sm := testLifecycleManager()
	now := time.Now()
	old := sm.newSession(&models.Events{SiteId: 1, UserId: 1, Name: "pageview", Timestamp: now.Add(-time.Minute)})
	got, err := sm.handleEvent(&models.Events{SiteId: 1, UserId: 1, Name: "engagement", Interactive: true, Timestamp: now}, old)
	if err != nil {
		t.Fatal(err)
	}
	if got.IsBounce != 1 || got.PageViews != 1 || got.Events != 2 {
		t.Fatalf("engagement changed bounce/pageview semantics: %+v", got)
	}
	if len(sm.writeBuffer.buffer) != 2 || sm.writeBuffer.buffer[0].Sign != -1 || sm.writeBuffer.buffer[0].Version != old.Version || got.Version <= old.Version {
		t.Fatal("session update did not produce matching tombstone and newer state")
	}
}
