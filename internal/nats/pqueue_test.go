package nats

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPersistentQueue(t *testing.T) {
	// Create a temporary database file
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	// Track processed events
	var mu sync.Mutex
	processedEvents := make(map[int]struct{})
	failCount := 0
	maxFails := 0

	// Worker that tracks processing
	worker := func(ctx context.Context, stream, subject string, payload []byte) error {
		mu.Lock()
		defer mu.Unlock()

		// Simulate failure for testing retry
		if failCount < maxFails {
			failCount++
			return fmt.Errorf("simulated failure")
		}

		// Extract event number from payload
		eventNum := int(payload[0])
		processedEvents[eventNum] = struct{}{}

		return nil
	}

	t.Run("Add and process events", func(t *testing.T) {
		pq, err := NewPersistentQueue(dbPath, worker)
		require.NoError(t, err)
		defer pq.Close()
		defer os.Remove(dbPath)

		// Add events
		for i := 0; i < 5; i++ {
			err := pq.Add("test-stream", "test.subject", []byte{byte(i)})
			require.NoError(t, err)
		}

		// Wait for events to be processed
		time.Sleep(2 * time.Second)

		// Check that all events were processed
		mu.Lock()
		assert.Len(t, processedEvents, 5)
		mu.Unlock()
	})

	t.Run("Retry on failure", func(t *testing.T) {
		// Reset tracking
		mu.Lock()
		processedEvents = make(map[int]struct{})
		failCount = 0
		maxFails = 2 // Fail twice before succeeding
		mu.Unlock()

		pq, err := NewPersistentQueue(dbPath, worker)
		require.NoError(t, err)
		defer pq.Close()
		defer os.Remove(dbPath)

		// Add an event
		err = pq.Add("test-stream", "test.subject", []byte{byte(42)})
		require.NoError(t, err)

		// Wait for retries and eventual success
		time.Sleep(25 * time.Second)

		// Check that the event was eventually processed
		mu.Lock()
		assert.Contains(t, processedEvents, 42)
		assert.Equal(t, maxFails, failCount)
		mu.Unlock()
	})

	t.Run("FIFO ordering", func(t *testing.T) {
		// Reset tracking
		mu.Lock()
		processedEvents = make(map[int]struct{})
		failCount = 0
		maxFails = 0
		mu.Unlock()

		var order []int
		var orderMu sync.Mutex

		orderedWorker := func(ctx context.Context, stream, subject string, payload []byte) error {
			orderMu.Lock()
			defer orderMu.Unlock()
			order = append(order, int(payload[0]))
			return nil
		}

		pq, err := NewPersistentQueue(dbPath, orderedWorker)
		require.NoError(t, err)
		defer pq.Close()
		defer os.Remove(dbPath)

		// Add events in order
		for i := 0; i < 5; i++ {
			err := pq.Add("test-stream", "test.subject", []byte{byte(i)})
			require.NoError(t, err)
		}

		// Wait for all events to be processed
		time.Sleep(2 * time.Second)

		// Check FIFO ordering
		orderMu.Lock()
		assert.Equal(t, []int{0, 1, 2, 3, 4}, order)
		orderMu.Unlock()
	})

	t.Run("Size", func(t *testing.T) {
		pq, err := NewPersistentQueue(dbPath, worker)
		require.NoError(t, err)
		defer pq.Close()
		defer os.Remove(dbPath)

		// Check empty size
		size, err := pq.Size()
		require.NoError(t, err)
		assert.Equal(t, 0, size)

		// Add events
		for i := 0; i < 3; i++ {
			err := pq.Add("test-stream", "test.subject", []byte{byte(i)})
			require.NoError(t, err)
		}

		// Check size
		size, err = pq.Size()
		require.NoError(t, err)
		assert.Equal(t, 3, size)

		// Wait for processing
		time.Sleep(2 * time.Second)

		// Check size after processing
		size, err = pq.Size()
		require.NoError(t, err)
		assert.Equal(t, 0, size)
	})

	t.Run("Multiple workers not supported", func(t *testing.T) {
		// This test verifies that we don't have race conditions
		// when multiple goroutines add events
		pq, err := NewPersistentQueue(dbPath, worker)
		require.NoError(t, err)
		defer pq.Close()
		defer os.Remove(dbPath)

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				err := pq.Add("test-stream", "test.subject", []byte{byte(n)})
				assert.NoError(t, err)
			}(i)
		}
		wg.Wait()

		// Wait for processing
		time.Sleep(2 * time.Second)

		// Check that all events were processed
		mu.Lock()
		assert.Len(t, processedEvents, 10)
		mu.Unlock()
	})
}
