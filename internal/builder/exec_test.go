package builder

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"testing/synctest"
	"time"

	_ "net/http/pprof"

	"github.com/stretchr/testify/assert"
)

type RunnableDummy struct {
	result int
}

func (r *RunnableDummy) Run(ctx context.Context) error {
	r.result = 1
	return nil
}

func TestNewExec(t *testing.T) {
	r := &RunnableDummy{}
	e := NewExec(r, time.Second)
	assert.Equal(t, 0, r.result)
	e.Start(t.Context())
	e.Wait()
	assert.Equal(t, 1, r.result)
	assert.True(t, e.finished.Load())
	assert.Nil(t, e.getErr())
}

func TestExecCompletionSignalIsRetained(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := NewExec(&RunnableDummy{}, time.Second)
		e.Start(t.Context())
		synctest.Wait()

		// Wait can observe started before completion and receive afterwards.
		select {
		case <-e.done:
		default:
			t.Fatal("Exec completion signal was lost before the waiter received it")
		}
	})
}

type RunnableContext struct{}

func (r *RunnableContext) Run(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "sleep", "3")
	err := cmd.Run()
	return err
}
func TestExecTimeout(t *testing.T) {
	r := &RunnableContext{}
	e := NewExec(r, time.Second)
	e.Start(t.Context())
	e.Wait()
	assert.Equal(t, context.DeadlineExceeded, e.getErr())
}

func TestExecStop(t *testing.T) {
	r := &RunnableContext{}
	e := NewExec(r, 5*time.Second)
	e.Start(t.Context())
	time.Sleep(500 * time.Millisecond)
	e.Stop()
	e.Wait()
	assert.Equal(t, context.Canceled, e.getErr())
}

type RunnableError struct{}

func (r *RunnableError) Run(ctx context.Context) error {
	return fmt.Errorf("An error occurred")
}
func TestExecError(t *testing.T) {
	r := &RunnableError{}
	e := NewExec(r, 5*time.Second)
	e.Start(t.Context())
	e.Wait()
	assert.True(t, e.finished.Load())
	assert.ErrorContains(t, e.getErr(), "An error occurred")
}
