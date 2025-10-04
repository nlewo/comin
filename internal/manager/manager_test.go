package manager

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/nlewo/comin/internal/builder"
	"github.com/nlewo/comin/internal/deployer"
	"github.com/nlewo/comin/internal/executor"
	"github.com/nlewo/comin/internal/fetcher"
	"github.com/nlewo/comin/internal/prometheus"
	"github.com/nlewo/comin/internal/protobuf"
	"github.com/nlewo/comin/internal/scheduler"
	"github.com/nlewo/comin/internal/store"
	"github.com/nlewo/comin/internal/utils"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

var mkDeployerMock = func(t *testing.T) *deployer.Deployer {
	var deployFunc = func(context.Context, string, string) (bool, string, error) {
		return false, "", nil
	}
	tmp := t.TempDir()
	s, err := store.New(tmp+"/state.json", tmp+"/gcroots", 1, 1)
	assert.Nil(t, err)
	return deployer.New(s, deployFunc, nil, "")
}

type ExecutorMock struct {
	evalOk    chan bool
	buildOk   chan bool
	machineId string
}

func (n ExecutorMock) ReadMachineId() (string, error) {
	return "", nil
}
func (n ExecutorMock) NeedToReboot() bool {
	return false
}
func (n ExecutorMock) IsStorePathExist(storePath string) bool {
	return false
}
func (n ExecutorMock) Deploy(ctx context.Context, outPath, operation string) (needToRestartComin bool, profilePath string, err error) {
	return false, "", nil
}
func (n ExecutorMock) Eval(ctx context.Context, flakeUrl, hostname string) (drvPath string, outPath string, machineId string, err error) {
	ok := <-n.evalOk
	if ok {
		return "drv-path", "out-path", n.machineId, nil
	} else {
		return "", "", n.machineId, fmt.Errorf("An error occured")
	}
}
func (n ExecutorMock) Build(ctx context.Context, drvPath string) (err error) {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case ok := <-n.buildOk:
		if ok {
			return nil
		} else {
			return fmt.Errorf("An error occured")
		}
	}
}
func NewExecutorMock(machineId string) ExecutorMock {
	return ExecutorMock{
		evalOk:  make(chan bool, 1),
		buildOk: make(chan bool, 1),
	}
}

func TestBuild(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	r := utils.NewRepositoryMock()
	f := fetcher.NewFetcher(r)
	tmp := t.TempDir()
	s, _ := store.New(tmp+"/state.json", tmp+"/gcroots", 1, 1)
	f.Start()
	eMock := NewExecutorMock("")
	b := builder.New(s, eMock, "repoPath", "", "my-machine", 2*time.Second, 2*time.Second)
	var deployFunc = func(context.Context, string, string) (bool, string, error) {
		return false, "profile-path", nil
	}
	d := deployer.New(s, deployFunc, nil, "")
	e, _ := executor.NewNixOS()
	m := New(s, prometheus.New(), scheduler.New(), f, b, d, "", e)
	go m.Run()
	assert.False(t, m.Fetcher.GetState().IsFetching.GetValue())
	assert.False(t, m.Builder.State().IsEvaluating.GetValue())
	assert.False(t, m.Builder.State().IsBuilding.GetValue())

	commitId := "id-1"
	f.TriggerFetch([]string{"remote"})
	r.RsCh <- &protobuf.RepositoryStatus{
		SelectedCommitId: commitId,
	}
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.True(c, m.Builder.State().IsEvaluating.GetValue())
		assert.False(c, m.Builder.State().IsBuilding.GetValue())
	}, 5*time.Second, 100*time.Millisecond)

	// This simulates the failure of an evaluation
	eMock.evalOk <- false
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.False(c, m.Builder.State().IsEvaluating.GetValue())
		assert.False(c, m.Builder.State().IsBuilding.GetValue())
		g, _ := m.storage.GenerationGet(m.Builder.GenerationUuid)
		assert.NotNil(c, g.EvalErr)
		assert.Nil(c, m.deployer.GenerationToDeploy)
	}, 5*time.Second, 100*time.Millisecond)

	commitId = "id-2"
	f.TriggerFetch([]string{"remote"})
	r.RsCh <- &protobuf.RepositoryStatus{
		SelectedCommitId: commitId,
	}
	// This simulates the success of an evaluation
	eMock.evalOk <- true
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.False(c, m.Builder.State().IsEvaluating.GetValue())
		assert.True(c, m.Builder.State().IsBuilding.GetValue())
		g, _ := m.storage.GenerationGet(m.Builder.GenerationUuid)
		assert.Empty(c, g.EvalErr)
		assert.Nil(c, m.deployer.GenerationToDeploy)
	}, 5*time.Second, 100*time.Millisecond)

	// This simulates the failure of a build
	eMock.buildOk <- false
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.False(c, m.Builder.State().IsEvaluating.GetValue())
		assert.False(c, m.Builder.State().IsBuilding.GetValue())
		g, _ := m.storage.GenerationGet(m.Builder.GenerationUuid)
		assert.NotNil(c, g.BuildErr)
		assert.Nil(c, m.deployer.GenerationToDeploy)
	}, 5*time.Second, 100*time.Millisecond)

	// This simulates the success of a build
	f.TriggerFetch([]string{"remote"})
	r.RsCh <- &protobuf.RepositoryStatus{
		SelectedCommitId: "id-3",
	}
	eMock.evalOk <- true
	eMock.buildOk <- true
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.False(c, m.Builder.State().IsEvaluating.GetValue())
		assert.False(c, m.Builder.State().IsBuilding.GetValue())
		g, _ := m.storage.GenerationGet(m.Builder.GenerationUuid)
		assert.Empty(c, g.BuildErr)
	}, 5*time.Second, 100*time.Millisecond)

	// This simulates the success of another build and ensure this
	// new build is the one proposed for deployment.
	f.TriggerFetch([]string{"remote"})
	r.RsCh <- &protobuf.RepositoryStatus{
		SelectedCommitId: "id-4",
	}
	eMock.evalOk <- true
	eMock.buildOk <- true
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.False(c, m.Builder.State().IsEvaluating.GetValue())
		assert.False(c, m.Builder.State().IsBuilding.GetValue())
		g, _ := m.storage.GenerationGet(m.Builder.GenerationUuid)
		assert.Empty(c, g.BuildErr)
	}, 5*time.Second, 100*time.Millisecond)

	// This simulates the push of new commit while building
	f.TriggerFetch([]string{"remote"})
	r.RsCh <- &protobuf.RepositoryStatus{
		SelectedCommitId: "id-5",
	}
	eMock.evalOk <- true
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.True(c, m.Builder.State().IsBuilding.GetValue())
	}, 5*time.Second, 100*time.Millisecond)
}

func TestDeploy(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	r := utils.NewRepositoryMock()
	f := fetcher.NewFetcher(r)
	f.Start()
	tmp := t.TempDir()
	s, _ := store.New(tmp+"/state.json", tmp+"/gcroots", 1, 1)
	eMock := NewExecutorMock("")
	eMock.evalOk <- true
	eMock.buildOk <- true
	b := builder.New(s, eMock, "repoPath", "", "my-machine", 2*time.Second, 2*time.Second)
	var deployFunc = func(context.Context, string, string) (bool, string, error) {
		return false, "profile-path", nil
	}
	d := deployer.New(s, deployFunc, nil, "")
	e, _ := executor.NewNixOS()
	m := New(s, prometheus.New(), scheduler.New(), f, b, d, "", e)
	go m.Run()
	assert.False(t, m.Fetcher.GetState().IsFetching.GetValue())
	assert.False(t, m.Builder.State().IsEvaluating.GetValue())
	assert.False(t, m.Builder.State().IsBuilding.GetValue())
	m.deployer.Submit(&protobuf.Generation{})
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, "profile-path", m.deployer.State().Deployment.ProfilePath)
	}, 5*time.Second, 100*time.Millisecond)

}

func TestIncorrectMachineId(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	r := utils.NewRepositoryMock()
	f := fetcher.NewFetcher(r)
	f.Start()
	tmp := t.TempDir()
	s, _ := store.New(tmp+"/state.json", tmp+"/gcroots", 1, 1)
	eMock := NewExecutorMock("invalid-machine-id")
	b := builder.New(s, eMock, "repoPath", "", "my-machine", 2*time.Second, 2*time.Second)
	d := mkDeployerMock(t)
	e, _ := executor.NewNixOS()
	m := New(s, prometheus.New(), scheduler.New(), f, b, d, "the-test-machine-id", e)
	go m.Run()

	f.TriggerFetch([]string{"remote"})
	r.RsCh <- &protobuf.RepositoryStatus{
		SelectedCommitId: "id",
	}

	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.False(t, m.GetState().Builder.IsBuilding.GetValue())
	}, 5*time.Second, 100*time.Millisecond)
}

func TestCorrectMachineId(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	r := utils.NewRepositoryMock()
	f := fetcher.NewFetcher(r)
	f.Start()
	tmp := t.TempDir()
	s, _ := store.New(tmp+"/state.json", tmp+"/gcroots", 1, 1)
	eMock := NewExecutorMock("the-test-machine-id")
	eMock.evalOk <- true
	b := builder.New(s, eMock, "repoPath", "", "my-machine", 2*time.Second, 2*time.Second)
	d := mkDeployerMock(t)
	e, _ := executor.NewNixOS()
	m := New(s, prometheus.New(), scheduler.New(), f, b, d, "the-test-machine-id", e)
	go m.Run()

	f.TriggerFetch([]string{"remote"})
	r.RsCh <- &protobuf.RepositoryStatus{
		SelectedCommitId: "id",
	}

	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.True(t, m.GetState().Builder.IsBuilding.GetValue())
	}, 5*time.Second, 100*time.Millisecond)
}

func TestManagerWithDarwinConfiguration(t *testing.T) {
	r := utils.NewRepositoryMock()
	f := fetcher.NewFetcher(r)
	tmp := t.TempDir()
	eMock := NewExecutorMock("")
	eMock.buildOk <- true
	s, _ := store.New(tmp+"/state.json", tmp+"/gcroots", 1, 1)
	b := builder.New(s, eMock, "repoPath", "", "my-machine", 2*time.Second, 2*time.Second)
	d := mkDeployerMock(t)

	// Test with Darwin configuration
	e, _ := executor.NewNixDarwin()
	m := New(s, prometheus.New(), scheduler.New(), f, b, d, "darwin-machine-id", e)

	// Verify the manager was created with the correct configuration attribute
	assert.Equal(t, "darwin-machine-id", m.machineId)

	// Verify the Darwin manager functions correctly without errors
	state := m.toState()
	assert.NotNil(t, state)
	assert.Equal(t, "darwin-machine-id", m.machineId)
}
