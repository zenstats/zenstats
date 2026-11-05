// Package ingestion records process-local collection quality without depending
// on the analytics databases. Counts describe events, not HTTP response codes.
package ingestion

import (
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

const HistoryMinutes = 60

var counterNames = []string{"requests", "invalid_requests", "received", "accepted", "filtered", "rejected", "buffered", "persisted", "failed"}

var Default = NewMonitor()

type Counters map[string]uint64

type Minute struct {
	Timestamp time.Time `json:"timestamp"`
	Counters  Counters  `json:"counters"`
}

type Reason struct {
	Stage   string `json:"stage"`
	Outcome string `json:"outcome"`
	Reason  string `json:"reason"`
	Count   uint64 `json:"count"`
}

// WriterStats counts physical rows: session versions and tombstones are not
// events. FailedRows means unconfirmed writes, including ambiguous timeouts.
type WriterStats struct {
	QueuedRows       int64      `json:"queued_rows"`
	InflightRows     int64      `json:"inflight_rows"`
	WrittenRows      uint64     `json:"written_rows"`
	FailedRows       uint64     `json:"failed_rows"`
	RetryAttempts    uint64     `json:"retry_attempts"`
	FailedAttempts   uint64     `json:"failed_attempts"`
	SucceededBatches uint64     `json:"succeeded_batches"`
	FailedBatches    uint64     `json:"failed_batches"`
	LastSuccessAt    *time.Time `json:"last_success_at"`
	LastFailureAt    *time.Time `json:"last_failure_at"`
	LastDurationMS   float64    `json:"last_duration_ms"`
}

type Pending struct {
	Events       int            `json:"events"`
	OldestAgeSec float64        `json:"oldest_age_seconds"`
	Stages       map[string]int `json:"stages"`
}

type Snapshot struct {
	InstanceID string                 `json:"instance_id"`
	StartedAt  time.Time              `json:"started_at"`
	Timestamp  time.Time              `json:"timestamp"`
	Counters   Counters               `json:"counters"`
	Pending    Pending                `json:"pending"`
	Reasons    []Reason               `json:"reasons"`
	Writers    map[string]WriterStats `json:"writers"`
	History    []Minute               `json:"history"`
}

type pendingEvent struct {
	at    time.Time
	stage string
}

type Monitor struct {
	mu         sync.Mutex
	instanceID string
	startedAt  time.Time
	nextID     uint64
	counters   Counters
	reasons    map[[3]string]uint64
	pending    map[uint64]pendingEvent
	writers    map[string]*WriterStats
	history    [HistoryMinutes]Minute
}

// Record follows a live API event through the worker and events writer. It is
// never serialized. A terminal outcome can be recorded only once.
type Record struct {
	monitor *Monitor
	id      uint64
}

func NewMonitor() *Monitor {
	return &Monitor{instanceID: uuid.NewString(), startedAt: time.Now().UTC(),
		counters: emptyCounters(), reasons: make(map[[3]string]uint64), pending: make(map[uint64]pendingEvent),
		writers: map[string]*WriterStats{"events": {}, "sessions": {}}}
}

func emptyCounters() Counters {
	c := make(Counters, len(counterNames))
	for _, name := range counterNames {
		c[name] = 0
	}
	return c
}

func cloneCounters(c Counters) Counters {
	copy := emptyCounters()
	for k, v := range c {
		copy[k] = v
	}
	return copy
}

func (m *Monitor) countLocked(name string, n uint64, now time.Time) {
	m.counters[name] += n
	minute := now.UTC().Truncate(time.Minute)
	i := minute.Unix() / 60 % HistoryMinutes
	if !m.history[i].Timestamp.Equal(minute) {
		m.history[i] = Minute{Timestamp: minute, Counters: emptyCounters()}
	}
	m.history[i].Counters[name] += n
}

func (m *Monitor) Count(name string, n int) {
	if n <= 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.countLocked(name, uint64(n), time.Now())
}

func (m *Monitor) Outcome(stage, outcome, reason string, n int) {
	if n <= 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.countLocked(outcome, uint64(n), time.Now())
	m.reasons[[3]string{stage, outcome, reason}] += uint64(n)
}

// Accept publishes counters and tracking records atomically with queue admission.
// enqueue must only attach records and enqueue; it must not call this monitor.
// Its all-or-nothing contract prevents partial batches being retried as a whole.
func (m *Monitor) Accept(n int, enqueue func([]*Record) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	records := make([]*Record, n)
	for i := range records {
		m.nextID++
		records[i] = &Record{monitor: m, id: m.nextID}
		m.pending[m.nextID] = pendingEvent{at: now, stage: "queued"}
	}
	if err := enqueue(records); err != nil {
		for _, r := range records {
			delete(m.pending, r.id)
		}
		return err
	}
	m.countLocked("accepted", uint64(n), now)
	return nil
}

func (r *Record) Stage(stage string) {
	if r == nil {
		return
	}
	m := r.monitor
	m.mu.Lock()
	defer m.mu.Unlock()
	if pending, ok := m.pending[r.id]; ok {
		if stage == "buffered" && pending.stage != "buffered" {
			m.countLocked("buffered", 1, time.Now())
		}
		pending.stage = stage
		m.pending[r.id] = pending
	}
}

func (r *Record) Finish(outcome, reason string) {
	if r == nil {
		return
	}
	m := r.monitor
	m.mu.Lock()
	defer m.mu.Unlock()
	if pending, ok := m.pending[r.id]; ok {
		delete(m.pending, r.id)
		m.countLocked(outcome, 1, time.Now())
		if reason != "" {
			m.reasons[[3]string{pending.stage, outcome, reason}]++
		}
	}
}

func (m *Monitor) QueueRows(table string, n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writers[table].QueuedRows += int64(n)
}

func (m *Monitor) StartWrite(table string, n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	w := m.writers[table]
	w.QueuedRows -= int64(n)
	w.InflightRows += int64(n)
}

func (m *Monitor) WriteAttempt(table string, retry, failed bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	w := m.writers[table]
	if retry {
		w.RetryAttempts++
	}
	if failed {
		w.FailedAttempts++
		now := time.Now().UTC()
		w.LastFailureAt = &now
	}
}

func (m *Monitor) FinishWrite(table string, n int, duration time.Duration, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	w := m.writers[table]
	w.InflightRows -= int64(n)
	w.LastDurationMS = float64(duration.Microseconds()) / 1000
	now := time.Now().UTC()
	if err != nil {
		w.FailedRows += uint64(n)
		w.FailedBatches++
		w.LastFailureAt = &now
	} else {
		w.WrittenRows += uint64(n)
		w.SucceededBatches++
		w.LastSuccessAt = &now
	}
}

func (m *Monitor) Snapshot() Snapshot {
	return m.snapshotAt(time.Now().UTC())
}

func (m *Monitor) snapshotAt(now time.Time) Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := Snapshot{InstanceID: m.instanceID, StartedAt: m.startedAt, Timestamp: now,
		Counters: cloneCounters(m.counters), Pending: Pending{Events: len(m.pending), Stages: map[string]int{}},
		Reasons: []Reason{}, Writers: map[string]WriterStats{}, History: []Minute{}}
	for _, p := range m.pending {
		s.Pending.Stages[p.stage]++
		if age := now.Sub(p.at).Seconds(); age > s.Pending.OldestAgeSec {
			s.Pending.OldestAgeSec = age
		}
	}
	for key, count := range m.reasons {
		s.Reasons = append(s.Reasons, Reason{Stage: key[0], Outcome: key[1], Reason: key[2], Count: count})
	}
	sort.Slice(s.Reasons, func(i, j int) bool {
		a, b := s.Reasons[i], s.Reasons[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		return a.Stage+a.Outcome+a.Reason < b.Stage+b.Outcome+b.Reason
	})
	for table, writer := range m.writers {
		copy := *writer
		if writer.LastSuccessAt != nil {
			at := *writer.LastSuccessAt
			copy.LastSuccessAt = &at
		}
		if writer.LastFailureAt != nil {
			at := *writer.LastFailureAt
			copy.LastFailureAt = &at
		}
		s.Writers[table] = copy
	}
	for i := HistoryMinutes - 1; i >= 0; i-- {
		at := now.Truncate(time.Minute).Add(-time.Duration(i) * time.Minute)
		bucket := m.history[at.Unix()/60%HistoryMinutes]
		counts := emptyCounters()
		if bucket.Timestamp.Equal(at) {
			counts = cloneCounters(bucket.Counters)
		}
		s.History = append(s.History, Minute{Timestamp: at, Counters: counts})
	}
	return s
}
