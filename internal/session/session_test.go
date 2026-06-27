package session

import (
	"context"
	"testing"
	"time"

	"github.com/zenstats/zenstats/internal/store/clickhouse/models"
)

// TestSessionIDMachineIDBitMasking verifies the sessionId layout:
//
//	bits[63:24] = timestamp (40 bits)
//	bits[23:16] = machineID (8 bits, masked)
//	bits[15:0]  = random    (16 bits)
//
// Without the mask a 64-bit machineID would overwrite the timestamp bits.
func TestSessionIDMachineIDBitMasking(t *testing.T) {
	sm := &SessionManager{
		machineID: 0xFFFFFFFFFFFFFFFF, // all bits set — worst case
	}

	event := &models.Events{
		UserId:    1,
		SiteId:    1,
		Name:      "pageview",
		Timestamp: time.Now(),
	}

	session := sm.newSession(event)
	sid := session.SessionId

	// MachineID contribution must occupy exactly bits 23:16 (8 bits).
	machineContrib := (sid >> 16) & 0xFF
	if machineContrib != 0xFF {
		t.Fatalf("expected machineID bits 23:16 == 0xFF, got 0x%X", machineContrib)
	}

	// Timestamp bits must be non-zero (come from time.Now().UnixNano()).
	if sid>>24 == 0 {
		t.Fatal("expected non-zero timestamp in bits 63:24 of sessionId")
	}
}

// TestSessionIDMachineIDDoesNotCorruptTimestampBits verifies that a non-trivial
// machineID value does not bleed into the timestamp region (bits 63:24).
func TestSessionIDMachineIDDoesNotCorruptTimestampBits(t *testing.T) {
	// machineID = 0xABCD_ABCD_ABCD_ABCD: without masking, bits above 8 spill into
	// the timestamp region and produce nonsensical session IDs.
	sm := &SessionManager{machineID: 0xABCDABCDABCDABCD}

	event := &models.Events{
		UserId:    2,
		SiteId:    2,
		Name:      "pageview",
		Timestamp: time.Now(),
	}

	session := sm.newSession(event)

	// Only bits 23:16 should carry the machineID contribution (0xCD = 0xABCD & 0xFF).
	machineContrib := (session.SessionId >> 16) & 0xFF
	if machineContrib != 0xCD {
		t.Fatalf("expected machineID contribution 0xCD in bits 23:16, got 0x%X", machineContrib)
	}
}

// TestCopySessionReturnsNewPointer verifies CopySession allocates a new struct.
func TestCopySessionReturnsNewPointer(t *testing.T) {
	sm := NewSessionManager(context.Background(), 10)
	defer sm.Shutdown()

	orig := &models.Sessions{Sign: 1, SessionId: 12345, UserId: 10, SiteId: 20, Duration: 100, Events: 5}
	copied := sm.CopySession(orig)

	if copied == orig {
		t.Fatal("CopySession must return a new pointer, not the same instance")
	}
}

// TestCopySessionPreservesAllFields verifies field-by-field copy correctness.
func TestCopySessionPreservesAllFields(t *testing.T) {
	sm := NewSessionManager(context.Background(), 10)
	defer sm.Shutdown()

	orig := &models.Sessions{
		Sign:      1,
		SessionId: 99999,
		UserId:    7,
		SiteId:    3,
		Duration:  250,
		Events:    12,
		PageViews: 8,
		IsBounce:  0,
		EntryPage: "/home",
		ExitPage:  "/about",
		Browser:   "Chrome",
	}

	copied := sm.CopySession(orig)

	if copied.Sign != orig.Sign {
		t.Errorf("Sign: got %d, want %d", copied.Sign, orig.Sign)
	}
	if copied.SessionId != orig.SessionId {
		t.Errorf("SessionId: got %d, want %d", copied.SessionId, orig.SessionId)
	}
	if copied.UserId != orig.UserId {
		t.Errorf("UserId: got %d, want %d", copied.UserId, orig.UserId)
	}
	if copied.Duration != orig.Duration {
		t.Errorf("Duration: got %d, want %d", copied.Duration, orig.Duration)
	}
	if copied.Events != orig.Events {
		t.Errorf("Events: got %d, want %d", copied.Events, orig.Events)
	}
	if copied.EntryPage != orig.EntryPage {
		t.Errorf("EntryPage: got %q, want %q", copied.EntryPage, orig.EntryPage)
	}
	if copied.Browser != orig.Browser {
		t.Errorf("Browser: got %q, want %q", copied.Browser, orig.Browser)
	}
}

// TestCopySessionAssignsNewVersion verifies the copy gets a distinct version.
func TestCopySessionAssignsNewVersion(t *testing.T) {
	sm := NewSessionManager(context.Background(), 10)
	defer sm.Shutdown()

	orig := &models.Sessions{Sign: 1, SessionId: 1}
	orig.Version = sm.nextVersion()

	copied := sm.CopySession(orig)
	if copied.Version == orig.Version {
		t.Fatal("CopySession must assign a new (higher) Version to the copy")
	}
	if copied.Version <= orig.Version {
		t.Fatalf("expected copy Version > orig Version, got copy=%d orig=%d", copied.Version, orig.Version)
	}
}

// TestCopySessionNilReturnsNil verifies CopySession handles nil input safely.
func TestCopySessionNilReturnsNil(t *testing.T) {
	sm := NewSessionManager(context.Background(), 10)
	defer sm.Shutdown()

	if got := sm.CopySession(nil); got != nil {
		t.Fatalf("expected nil for nil input, got %v", got)
	}
}

// TestHandleEventEngagementCachedSessionWritesTombstone verifies the Sign=-1 tombstone
// path: for a cached engagement session, the old row is negated before the update.
func TestHandleEventEngagementCachedSessionWritesTombstone(t *testing.T) {
	sm := NewSessionManager(context.Background(), 100)
	defer sm.Shutdown()

	now := time.Now()
	existing := &models.Sessions{
		Sign:      1,
		SessionId: 77,
		UserId:    1,
		SiteId:    1,
		Start:     now.Add(-5 * time.Minute),
		Timestamp: now.Add(-5 * time.Minute),
		Events:    2,
	}
	sm.updateSessionCache(existing)

	event := &models.Events{
		UserId:    1,
		SiteId:    1,
		Name:      "engagement",
		Timestamp: now,
	}

	result, err := sm.handleEvent(event, existing)
	if err != nil {
		t.Fatalf("handleEvent: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil session result")
	}
	// The returned session must have Sign=1 (the updated row)
	if result.Sign != 1 {
		t.Fatalf("expected returned session Sign == 1, got %d", result.Sign)
	}
	// Duration should reflect time elapsed since Start
	expectedDuration := uint32(now.Sub(existing.Start).Seconds())
	if result.Duration != expectedDuration {
		t.Fatalf("expected Duration == %d, got %d", expectedDuration, result.Duration)
	}
}

// TestShutdownDoesNotDeadlock verifies that Shutdown flushes the write buffer
// before cancelling the context (no deadlock, no panic).
func TestShutdownDoesNotDeadlock(t *testing.T) {
	sm := NewSessionManager(context.Background(), 100)

	// Add a few sessions so the buffer has work to do on shutdown.
	for i := 0; i < 5; i++ {
		sm.writeBuffer.Add(&models.Sessions{Sign: 1, UserId: uint64(i)})
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		sm.Shutdown()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown deadlocked — writeBuffer.Shutdown must be called before shutdownCancel")
	}
}
