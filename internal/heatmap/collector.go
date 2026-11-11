package heatmap

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

var ErrQueueFull = errors.New("heatmap queue unavailable")

type Writer interface {
	Insert(context.Context, []Row) error
}

type Quality struct {
	InstanceID       string     `json:"instance_id"`
	StartedAt        time.Time  `json:"started_at"`
	Received         uint64     `json:"received"`
	Accepted         uint64     `json:"accepted"`
	Rejected         uint64     `json:"rejected"`
	Filtered         uint64     `json:"filtered"`
	Persisted        uint64     `json:"persisted"`
	Failed           uint64     `json:"failed"`
	Retries          uint64     `json:"retries"`
	Pending          int        `json:"pending"`
	OldestAgeSeconds float64    `json:"oldest_age_seconds"`
	LastSuccess      *time.Time `json:"last_success"`
	LastFailure      *time.Time `json:"last_failure"`
}

type queuedBatch[T any] struct {
	id   uint64
	rows []T
}

// Collector has its own bounded queue and one writer. Ordinary event/session
// workers never wait for this queue and heatmap writes cannot spawn unbounded work.
type Collector = BatchCollector[Row]

// BatchCollector shares the bounded transport and quality contract across independent datasets.
type BatchCollector[T any] struct {
	mu      sync.Mutex
	queue   chan queuedBatch[T]
	closed  bool
	next    uint64
	pending map[uint64]time.Time
	quality Quality
	writer  BatchWriter[T]
	done    chan struct{}
}

type BatchWriter[T any] interface {
	Insert(context.Context, []T) error
}

func NewCollector(writer Writer) *Collector { return NewBatchCollector[Row](writer) }

func NewBatchCollector[T any](writer BatchWriter[T]) *BatchCollector[T] {
	c := &BatchCollector[T]{queue: make(chan queuedBatch[T], 128), pending: make(map[uint64]time.Time), writer: writer, done: make(chan struct{})}
	c.quality.InstanceID = uuid.NewString()
	c.quality.StartedAt = time.Now().UTC()
	go c.run()
	return c
}

func (c *BatchCollector[T]) Received(n int) {
	c.mu.Lock()
	c.quality.Received += uint64(n)
	c.mu.Unlock()
}
func (c *BatchCollector[T]) Rejected(n int) {
	c.mu.Lock()
	c.quality.Rejected += uint64(n)
	c.mu.Unlock()
}
func (c *BatchCollector[T]) Filtered(n int) {
	c.mu.Lock()
	c.quality.Filtered += uint64(n)
	c.mu.Unlock()
}

func (c *BatchCollector[T]) Enqueue(rows []T) error {
	if len(rows) == 0 || len(rows) > MaxBatchSize {
		return errors.New("invalid click batch size")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		c.quality.Rejected += uint64(len(rows))
		return ErrQueueFull
	}
	c.next++
	item := queuedBatch[T]{id: c.next, rows: rows}
	select {
	case c.queue <- item:
		c.pending[item.id] = time.Now()
		c.quality.Pending += len(rows)
		c.quality.Accepted += uint64(len(rows))
		return nil
	default:
		c.quality.Rejected += uint64(len(rows))
		return ErrQueueFull
	}
}

func (c *BatchCollector[T]) Snapshot() Quality {
	c.mu.Lock()
	defer c.mu.Unlock()
	q := c.quality
	for _, at := range c.pending {
		if age := time.Since(at).Seconds(); age > q.OldestAgeSeconds {
			q.OldestAgeSeconds = age
		}
	}
	return q
}

func (c *BatchCollector[T]) Shutdown() {
	c.mu.Lock()
	if !c.closed {
		c.closed = true
		close(c.queue)
	}
	c.mu.Unlock()
	<-c.done
}

func (c *BatchCollector[T]) run() {
	defer close(c.done)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	items := []queuedBatch[T]{}
	rows := []T{}
	flush := func() {
		if len(rows) == 0 {
			return
		}
		var err error
		for attempt := 0; attempt < 3; attempt++ {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			err = c.writer.Insert(ctx, rows)
			cancel()
			if err == nil {
				break
			}
			if attempt < 2 {
				c.mu.Lock()
				c.quality.Retries++
				c.mu.Unlock()
				time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
			}
		}
		now := time.Now()
		c.mu.Lock()
		if err == nil {
			c.quality.Persisted += uint64(len(rows))
			c.quality.LastSuccess = &now
		} else {
			c.quality.Failed += uint64(len(rows))
			c.quality.LastFailure = &now
		}
		c.quality.Pending -= len(rows)
		for _, item := range items {
			delete(c.pending, item.id)
		}
		c.mu.Unlock()
		if err != nil {
			slog.Error("heatmap batch write failed", "rows", len(rows), "error", err)
		}
		items = nil
		rows = nil
	}
	for {
		select {
		case item, ok := <-c.queue:
			if !ok {
				flush()
				return
			}
			items = append(items, item)
			rows = append(rows, item.rows...)
			if len(rows) >= 256 {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}
