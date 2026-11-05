package ingestion

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestEventLifecycleAndFailedAdmission(t *testing.T) {
	m := NewMonitor()
	errFull := errors.New("full")
	if err := m.Accept(3, func([]*Record) error { return errFull }); !errors.Is(err, errFull) {
		t.Fatal(err)
	}
	if s := m.Snapshot(); s.Pending.Events != 0 || s.Counters["accepted"] != 0 {
		t.Fatalf("failed admission leaked records: %+v", s)
	}
	var records []*Record
	if err := m.Accept(3, func(r []*Record) error { records = r; return nil }); err != nil {
		t.Fatal(err)
	}
	records[0].Stage("processing")
	records[0].Stage("buffered")
	records[0].Stage("writing")
	records[0].Finish("persisted", "")
	records[0].Finish("failed", "write_failed")
	records[0].Stage("buffered")
	records[1].Finish("filtered", "bot")
	records[2].Finish("failed", "buffer_full")
	s := m.Snapshot()
	if s.Pending.Events != 0 || s.Counters["accepted"] != 3 || s.Counters["persisted"] != 1 || s.Counters["failed"] != 1 || s.Counters["filtered"] != 1 || s.Counters["buffered"] != 1 {
		t.Fatalf("incorrect final outcomes: %+v", s.Counters)
	}
	if len(s.Reasons) != 2 {
		t.Fatalf("duplicate terminal reason: %+v", s.Reasons)
	}
}

func TestConcurrentCompletionAndSnapshots(t *testing.T) {
	m := NewMonitor()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var record *Record
			_ = m.Accept(1, func(r []*Record) error { record = r[0]; return nil })
			record.Stage("processing")
			_ = m.Snapshot()
			var finish sync.WaitGroup
			for j := 0; j < 4; j++ {
				finish.Add(1)
				go func() { defer finish.Done(); record.Finish("persisted", "") }()
			}
			finish.Wait()
		}()
	}
	wg.Wait()
	s := m.Snapshot()
	if s.Pending.Events != 0 || s.Counters["accepted"] != 100 || s.Counters["persisted"] != 100 {
		t.Fatalf("concurrent accounting: %+v", s.Counters)
	}
}

func TestHistoryWindowAndSnapshotIsolation(t *testing.T) {
	m := NewMonitor()
	now := time.Now().UTC().Truncate(time.Minute)
	m.mu.Lock()
	m.countLocked("received", 2, now.Add(-60*time.Minute))
	m.countLocked("received", 3, now.Add(-59*time.Minute))
	m.countLocked("received", 4, now)
	m.mu.Unlock()
	m.QueueRows("events", 2)
	m.StartWrite("events", 2)
	m.WriteAttempt("events", false, true)
	m.WriteAttempt("events", true, false)
	m.FinishWrite("events", 2, 10*time.Millisecond, nil)
	m.QueueRows("sessions", 3)
	m.StartWrite("sessions", 3)
	m.FinishWrite("sessions", 3, time.Second, errors.New("timeout"))
	s := m.snapshotAt(now)
	if len(s.History) != 60 || s.History[0].Counters["received"] != 3 || s.History[59].Counters["received"] != 4 || s.Counters["received"] != 9 {
		t.Fatalf("history did not rotate: %+v", s.History)
	}
	w := s.Writers["events"]
	if w.QueuedRows != 0 || w.InflightRows != 0 || w.WrittenRows != 2 || w.RetryAttempts != 1 || w.FailedAttempts != 1 || w.SucceededBatches != 1 || w.FailedRows != 0 {
		t.Fatalf("incorrect writer accounting: %+v", w)
	}
	if w := s.Writers["sessions"]; w.FailedRows != 3 || w.WrittenRows != 0 || w.InflightRows != 0 || w.FailedBatches != 1 {
		t.Fatalf("unconfirmed rows reported as written: %+v", w)
	}
	s.Counters["received"] = 999
	s.History[59].Counters["received"] = 999
	*w.LastSuccessAt = time.Time{}
	*w.LastFailureAt = time.Time{}
	s = m.snapshotAt(now)
	if s.Counters["received"] != 9 || s.History[59].Counters["received"] != 4 || s.Writers["events"].LastSuccessAt.IsZero() || s.Writers["events"].LastFailureAt.IsZero() {
		t.Fatal("snapshot mutation changed the monitor")
	}
	s = m.snapshotAt(now.Add(time.Hour))
	for _, minute := range s.History {
		if minute.Counters["received"] != 0 {
			t.Fatal("expired bucket retained in history")
		}
	}
}
