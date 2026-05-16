package nats

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
)

type PersistentQueue struct {
	db     *sql.DB
	worker func(ctx context.Context, stream, subject string, payload []byte) error
	sem    chan struct{}
}

type EventRow struct {
	ID        int64
	CreatedAt time.Time
	Stream    string
	Subject   string
	Payload   []byte
}

func NewPersistentQueue(dbPath string, worker func(ctx context.Context, stream, subject string, payload []byte) error) (*PersistentQueue, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Create events table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			stream TEXT NOT NULL,
			subject TEXT NOT NULL,
			payload BLOB NOT NULL
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create events table: %w", err)
	}

	// Create index on created_at for FIFO ordering
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_events_created_at ON events(created_at)`)
	if err != nil {
		return nil, fmt.Errorf("failed to create index: %w", err)
	}

	pq := &PersistentQueue{
		db:     db,
		worker: worker,
		sem:    make(chan struct{}, 1),
	}

	// Start the worker goroutine
	go pq.runWorker()

	return pq, nil
}

func (pq *PersistentQueue) Close() error {
	return pq.db.Close()
}

func (pq *PersistentQueue) Add(stream, subject string, payload []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := pq.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, "INSERT INTO events (stream, subject, payload) VALUES (?, ?, ?)", stream, subject, payload)
	if err != nil {
		return fmt.Errorf("failed to insert event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Signal that there's work to do (non-blocking)
	select {
	case pq.sem <- struct{}{}:
	default:
		// Semaphore already has a signal, worker will check for more events
	}

	logrus.Debugf("pqueue: added event to queue (stream=%s, subject=%s)", stream, subject)
	return nil
}

func (pq *PersistentQueue) runWorker() {
	for {
		// Wait for work to be available
		<-pq.sem

		// Get the oldest event
		event, count, err := pq.getNextEvent()
		if err != nil {
			logrus.Errorf("pqueue: failed to get next event: %s", err)
			// Re-add to semaphore to retry
			pq.sem <- struct{}{}
			time.Sleep(10 * time.Second)
			continue
		}

		if event == nil {
			logrus.Warn("pqueue: no event in the table while notified by the semaphose (this can occur when a retry has been executed while another new event triggered the queue consumption)")
			continue
		}
		logrus.Debugf("pqueue: processing event %d (%d pending events in the queue)", event.ID, count-1)

		// Process the event
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err = pq.worker(ctx, event.Stream, event.Subject, event.Payload)
		cancel()

		if err != nil {
			logrus.Errorf("pqueue: worker failed for event %d (stream=%s, subject=%s): %s", event.ID, event.Stream, event.Subject, err)
			// Re-add to semaphore to retry after 10 seconds
			go func() {
				time.Sleep(10 * time.Second)
				// FIXME: this doesn't work really well since the queue could be consumed by a new event while waiting for the retry. In this case, the queue is empty but we still notify the worker
				pq.sem <- struct{}{}
			}()
			continue
		}

		// Remove the event from the queue
		err = pq.removeEvent(event.ID)
		if err != nil {
			logrus.Errorf("pqueue: failed to remove event %d: %s", event.ID, err)
			// Even if removal fails, continue to next event
			// Don't re-add to semaphore since we want to try the next event
			continue
		}

		// Event was successfully processed and removed
		// Trigger next worker if there are more events
		pq.triggerNextIfAvailable()

		logrus.Debugf("pqueue: successfully processed and removed event %d", event.ID)
	}
}

func (pq *PersistentQueue) triggerNextIfAvailable() {
	// Check if there are more events
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var count int
	err := pq.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM events").Scan(&count)
	if err != nil {
		logrus.Errorf("pqueue: failed to check for more events: %s", err)
		return
	}

	if count > 0 {
		// There are more events, signal worker
		select {
		case pq.sem <- struct{}{}:
		default:
			// Semaphore already has a signal, don't block
		}
	}
}

func (pq *PersistentQueue) getNextEvent() (event *EventRow, count int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = pq.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM events").Scan(&count)
	if err != nil {
		return event, count, fmt.Errorf("failed to count events: %s", err)
	}

	row := pq.db.QueryRowContext(ctx, "SELECT id, created_at, stream, subject, payload FROM events ORDER BY created_at ASC LIMIT 1")

	var e EventRow
	err = row.Scan(&e.ID, &e.CreatedAt, &e.Stream, &e.Subject, &e.Payload)
	if err != nil {
		if err == sql.ErrNoRows {
			return event, count, nil
		}
		return event, count, fmt.Errorf("failed to scan event: %w", err)
	}

	return &e, count, nil
}

func (pq *PersistentQueue) removeEvent(id int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := pq.db.ExecContext(ctx, "DELETE FROM events WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete event: %w", err)
	}

	return nil
}

func (pq *PersistentQueue) Size() (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var count int
	err := pq.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM events").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count events: %w", err)
	}

	return count, nil
}
