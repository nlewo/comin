package builder

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/nlewo/comin/internal/protobuf"
	"github.com/nlewo/comin/internal/store"
	"github.com/stretchr/testify/assert"

	"net/http"
	_ "net/http/pprof"
)

type ExecutorMock struct {
	evalDone     chan struct{}
	buildDone    chan struct{}
	alreadyBuilt bool
}

func (n ExecutorMock) ReadMachineId() (string, error) {
	return "", nil
}
func (n ExecutorMock) NeedToReboot() bool {
	return false
}
func (n ExecutorMock) IsStorePathExist(storePath string) bool {
	return n.alreadyBuilt
}
func (n ExecutorMock) Deploy(ctx context.Context, outPath, operation string) (needToRestartComin bool, profilePath string, err error) {
	return false, "", nil
}
func (n ExecutorMock) Eval(ctx context.Context, flakeUrl, hostname string) (drvPath string, outPath string, machineId string, err error) {
	select {
	case <-ctx.Done():
		return "", "", "", ctx.Err()
	case <-n.evalDone:
		return "drv-path", "out-path", "", nil
	}
}
func (n ExecutorMock) Build(ctx context.Context, drvPath string) (err error) {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-n.buildDone:
		return nil
	}
}
func NewExecutorMock(alreadyBuilt bool) ExecutorMock {
	return ExecutorMock{
		evalDone:     make(chan struct{}),
		buildDone:    make(chan struct{}),
		alreadyBuilt: alreadyBuilt,
	}
}

func TestBuilderBuild(t *testing.T) {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	tmp := t.TempDir()
	s, err := store.New(tmp+"/state.json", tmp+"/gcroots", 1, 1)
	assert.Nil(t, err)
	eMock := NewExecutorMock(false)
	b := New(s, eMock, "", "", "my-machine", 2*time.Second, 2*time.Second)

	// Run the evaluator
	_ = b.Eval(&protobuf.RepositoryStatus{})
	gUUID := <-b.EvaluationDone // The evaluation timeouts
	assert.ErrorContains(t, b.build(gUUID), "the generation is not evaluated")

	_ = b.Eval(&protobuf.RepositoryStatus{})
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.True(c, b.isEvaluating.Load())
	}, 2*time.Second, 100*time.Millisecond)
	eMock.evalDone <- struct{}{}
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.False(c, b.isEvaluating.Load())
	}, 2*time.Second, 100*time.Millisecond)
	gUUID = <-b.EvaluationDone

	err = b.build(gUUID)
	assert.Nil(t, err)
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.True(c, b.isBuilding.Load())
	}, 2*time.Second, 100*time.Millisecond)
	err = b.build(gUUID)
	assert.ErrorContains(t, err, "the builder is already building")

	// Stop the evaluator and builder
	b.Stop()
	gUUID = <-b.BuildDone
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.False(c, b.isBuilding.Load())
		g, _ := b.store.GenerationGet(gUUID)
		assert.Contains(c, g.BuildErr, "context canceled")
	}, 2*time.Second, 100*time.Millisecond)

	// The builder timeouts
	err = b.build(gUUID)
	assert.Nil(t, err)
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		g, _ := b.store.GenerationGet(gUUID)
		assert.Contains(c, g.BuildErr, "context deadline exceeded")
	}, 3*time.Second, 100*time.Millisecond)

	// The builder succeeds
	err = b.build(gUUID)
	assert.Nil(t, err)
	eMock.buildDone <- struct{}{}
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.False(c, b.isBuilding.Load())
	}, 3*time.Second, 100*time.Millisecond)

	// The generation is already built
	err = b.build(gUUID)
	assert.ErrorContains(t, err, "the generation is already built")
}

func TestEval(t *testing.T) {
	tmp := t.TempDir()
	s, err := store.New(tmp+"/state.json", tmp+"/gcroots", 1, 1)
	assert.Nil(t, err)
	eMock := NewExecutorMock(false)
	b := New(s, eMock, "", "", "", 5*time.Second, 5*time.Second)
	_ = b.Eval(&protobuf.RepositoryStatus{})
	assert.True(t, b.isEvaluating.Load())
	eMock.evalDone <- struct{}{}
	gUUID := <-b.EvaluationDone
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.False(c, b.isEvaluating.Load())
		g, _ := b.store.GenerationGet(gUUID)
		assert.Equal(c, store.Evaluated.String(), g.EvalStatus)
		assert.Equal(c, "drv-path", g.DrvPath)
		assert.Equal(c, "out-path", g.OutPath)
	}, 2*time.Second, 100*time.Millisecond)
}

// TestEvalAlreadyBuilt tests the evaluation when the storepath has been already built.
func TestEvalAlreadyBuilt(t *testing.T) {
	tmp := t.TempDir()
	s, err := store.New(tmp+"/state.json", tmp+"/gcroots", 1, 1)
	assert.Nil(t, err)
	eMock := NewExecutorMock(true)
	b := New(s, eMock, "", "", "", 5*time.Second, 5*time.Second)
	_ = b.Eval(&protobuf.RepositoryStatus{})
	assert.True(t, b.IsEvaluating())

	eMock.evalDone <- struct{}{}
	gUUID := <-b.BuildDone
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.False(c, b.IsEvaluating())
		g, _ := b.store.GenerationGet(gUUID)
		assert.Equal(c, store.Evaluated.String(), g.EvalStatus)
		assert.Equal(c, "drv-path", g.DrvPath)
		assert.Equal(c, "out-path", g.OutPath)
		assert.Equal(c, store.Built.String(), g.BuildStatus)
	}, 2*time.Second, 100*time.Millisecond)
}

func TestBuilderPreemption(t *testing.T) {
	tmp := t.TempDir()
	s, err := store.New(tmp+"/state.json", tmp+"/gcroots", 1, 1)
	assert.Nil(t, err)
	eMock := NewExecutorMock(false)
	b := New(s, eMock, "", "", "", 5*time.Second, 5*time.Second)
	_ = b.Eval(&protobuf.RepositoryStatus{SelectedCommitId: "commit-1"})
	assert.True(t, b.isEvaluating.Load())
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		g, _ := b.store.GenerationGet(b.GenerationUuid)
		assert.Equal(c, "commit-1", g.SelectedCommitId)
	}, 2*time.Second, 100*time.Millisecond)

	_ = b.Eval(&protobuf.RepositoryStatus{SelectedCommitId: "commit-2"})
	assert.True(t, b.isEvaluating.Load())
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		g, _ := b.store.GenerationGet(b.GenerationUuid)
		assert.Equal(c, "commit-2", g.SelectedCommitId)
	}, 2*time.Second, 100*time.Millisecond)
}

func TestBuilderStop(t *testing.T) {
	tmp := t.TempDir()
	s, err := store.New(tmp+"/state.json", tmp+"/gcroots", 1, 1)
	assert.Nil(t, err)
	eMock := NewExecutorMock(false)
	b := New(s, eMock, "", "", "", 5*time.Second, 5*time.Second)
	_ = b.Eval(&protobuf.RepositoryStatus{})
	assert.True(t, b.isEvaluating.Load())
	b.Stop()
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		g, _ := b.store.GenerationGet(b.GenerationUuid)
		assert.Contains(c, g.EvalErr, "context canceled")
	}, 2*time.Second, 100*time.Millisecond)
}

func TestBuilderTimeout(t *testing.T) {
	tmp := t.TempDir()
	s, err := store.New(tmp+"/state.json", tmp+"/gcroots", 1, 1)
	assert.Nil(t, err)
	eMock := NewExecutorMock(false)
	b := New(s, eMock, "", "", "", 1*time.Second, 5*time.Second)
	_ = b.Eval(&protobuf.RepositoryStatus{})
	assert.True(t, b.isEvaluating.Load())
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		g, _ := b.store.GenerationGet(b.GenerationUuid)
		assert.Contains(c, g.EvalErr, "context deadline exceeded")
	}, 3*time.Second, 100*time.Millisecond, "builder timeout didn't work")
}

func TestBuilderSuspend(t *testing.T) {
	tmp := t.TempDir()
	s, err := store.New(tmp+"/state.json", tmp+"/gcroots", 1, 1)
	assert.Nil(t, err)
	eMock := NewExecutorMock(false)
	b := New(s, eMock, "", "", "", 1*time.Second, 5*time.Second)
	_ = b.Suspend()
	assert.True(t, b.isSuspended)
	_ = b.Eval(&protobuf.RepositoryStatus{})
	assert.True(t, b.isEvaluating.Load())

	eMock.evalDone <- struct{}{}
	gUUID := <-b.EvaluationDone
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.False(c, b.isEvaluating.Load())
	}, 3*time.Second, 100*time.Millisecond)

	g, _ := b.store.GenerationGet(gUUID)
	assert.Equal(t, store.Evaluated.String(), g.EvalStatus)
	assert.Equal(t, store.BuildInit.String(), g.BuildStatus)
	err = b.Resume()
	assert.Nil(t, err)
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.True(c, b.isBuilding.Load())
	}, 3*time.Second, 100*time.Millisecond)
}
