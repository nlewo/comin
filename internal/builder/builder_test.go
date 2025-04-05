package builder

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/nlewo/comin/internal/repository"
	"github.com/stretchr/testify/assert"

	"net/http"
	_ "net/http/pprof"
)

var mkNixEvalMock = func(evalDone chan struct{}) EvalFunc {
	return func(ctx context.Context, repositoryPath string, hostname string) (string, string, string, error) {
		select {
		case <-ctx.Done():
			return "", "", "", ctx.Err()
		case <-evalDone:
			return "drv-path", "out-path", "", nil
		}
	}
}

var mkNixBuildMock = func(buildDone chan struct{}) BuildFunc {
	return func(ctx context.Context, drvPath string) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-buildDone:
			return nil
		}
	}
}

var nixBuildMockNil = func(ctx context.Context, drvPath string) error { return nil }

func TestBuilderBuild(t *testing.T) {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
	evalDone := make(chan struct{})
	buildDone := make(chan struct{})

	b := New("", "", "my-machine", 2*time.Second, mkNixEvalMock(evalDone), 2*time.Second, mkNixBuildMock(buildDone))

	assert.ErrorContains(t, b.Build(), "the generation is not evaluated")
	// Run the evaluator
	b.Eval(repository.RepositoryStatus{})
	close(evalDone)

	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.False(c, b.IsEvaluating)
	}, 2*time.Second, 100*time.Millisecond)

	_ = b.Build()
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.True(c, b.IsBuilding)
	}, 2*time.Second, 100*time.Millisecond)
	err := b.Build()
	assert.ErrorContains(t, err, "the builder is already building")

	// Stop the evaluator and builder
	b.Stop()

	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.False(c, b.IsBuilding)
		g := b.GetGeneration()
		assert.ErrorContains(c, g.BuildErr, "context canceled")
	}, 2*time.Second, 100*time.Millisecond)

	// The builder timeouts
	err = b.Build()
	assert.Nil(t, err)
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		g := b.GetGeneration()
		assert.ErrorContains(c, g.BuildErr, "context deadline exceeded")
	}, 3*time.Second, 100*time.Millisecond)

	// The builder succeeds
	err = b.Build()
	assert.Nil(t, err)
	buildDone <- struct{}{}
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.False(c, b.IsBuilding)
	}, 3*time.Second, 100*time.Millisecond)

	// The generation is already built
	err = b.Build()
	assert.ErrorContains(t, err, "the generation is already built")
}

func TestEval(t *testing.T) {
	evalDone := make(chan struct{})
	b := New("", "", "", 5*time.Second, mkNixEvalMock(evalDone), 5*time.Second, nixBuildMockNil)
	b.Eval(repository.RepositoryStatus{})
	assert.True(t, b.IsEvaluating)

	evalDone <- struct{}{}
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.False(c, b.IsEvaluating)
		g := b.GetGeneration()
		assert.Equal(c, Evaluated, g.EvalStatus)
		assert.Equal(c, "drv-path", g.DrvPath)
		assert.Equal(c, "out-path", g.OutPath)
	}, 2*time.Second, 100*time.Millisecond)
}

func TestBuilderPreemption(t *testing.T) {
	evalDone := make(chan struct{})
	b := New("", "", "", 5*time.Second, mkNixEvalMock(evalDone), 5*time.Second, nixBuildMockNil)
	b.Eval(repository.RepositoryStatus{SelectedCommitId: "commit-1"})
	assert.True(t, b.IsEvaluating)
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		g := b.GetGeneration()
		assert.Equal(c, "commit-1", g.SelectedCommitId)
	}, 2*time.Second, 100*time.Millisecond)

	b.Eval(repository.RepositoryStatus{SelectedCommitId: "commit-2"})
	assert.True(t, b.IsEvaluating)

	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		g := b.GetGeneration()
		assert.Equal(c, "commit-2", g.SelectedCommitId)
	}, 2*time.Second, 100*time.Millisecond)
}

func TestBuilderStop(t *testing.T) {
	evalDone := make(chan struct{})
	b := New("", "", "", 5*time.Second, mkNixEvalMock(evalDone), 5*time.Second, nixBuildMockNil)
	b.Eval(repository.RepositoryStatus{})
	assert.True(t, b.IsEvaluating)
	b.Stop()
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		g := b.GetGeneration()
		assert.ErrorContains(c, g.EvalErr, "context canceled")
	}, 2*time.Second, 100*time.Millisecond)
}

func TestBuilderTimeout(t *testing.T) {
	evalDone := make(chan struct{})
	b := New("", "", "", 1*time.Second, mkNixEvalMock(evalDone), 5*time.Second, nixBuildMockNil)
	b.Eval(repository.RepositoryStatus{})
	assert.True(t, b.IsEvaluating)
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		g := b.GetGeneration()
		assert.ErrorContains(c, g.EvalErr, "context deadline exceeded")
	}, 3*time.Second, 100*time.Millisecond, "builder timeout didn't work")
}
