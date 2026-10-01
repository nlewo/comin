package builder

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

type Runnable interface {
	Run(c context.Context) error
}

// Exec runs a runnable object asyncronously while recording start time, finish time and
type Exec struct {
	timeout  time.Duration
	runnable Runnable
	started  atomic.Bool
	finished atomic.Bool
	done     chan struct{}
	// err is used to know if the context has been cancelled,
	// timeouted or if the runnable ends with an error
	err        error
	cancelFunc context.CancelFunc
	mu         sync.Mutex
}

func NewExec(r Runnable, timeout time.Duration) Exec {
	return Exec{
		runnable: r,
		mu:       sync.Mutex{},
		done:     make(chan struct{}),
		timeout:  timeout,
	}
}

func (e *Exec) Start(ctx context.Context) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.started.Store(true)
	ctx, e.cancelFunc = context.WithCancel(ctx)
	ctx, cancel := context.WithTimeout(ctx, e.timeout)

	go func() {
		defer e.cancelFunc()
		defer cancel()
		err := e.runnable.Run(ctx)
		e.mu.Lock()
		defer e.mu.Unlock()
		if ctx.Err() != nil {
			e.err = ctx.Err()
		} else {
			e.err = err
		}
		e.started.Store(false)
		e.finished.Store(true)
		select {
		case e.done <- struct{}{}:
		default:
		}
	}()
}

func (e *Exec) Wait() {
	if e.started.Load() {
		<-e.done
	}
}

func (e *Exec) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.started.Load() {
		e.cancelFunc()
	}
}

// getErr returns the execution error in a thread-safe way
func (e *Exec) getErr() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.err
}
