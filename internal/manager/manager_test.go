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
	"github.com/nlewo/comin/internal/repository"
	"github.com/nlewo/comin/internal/scheduler"
	"github.com/nlewo/comin/internal/store"
	"github.com/nlewo/comin/internal/utils"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

var mkNixEvalMock = func(evalOk chan bool) builder.EvalFunc {
	return func(ctx context.Context, repositoryPath string, hostname string) (string, string, string, error) {
		ok := <-evalOk
		if ok {
			return "drv-path", "out-path", "", nil
		} else {
			return "", "", "", fmt.Errorf("An error occured")
		}
	}
}

var mkDeployerMock = func() *deployer.Deployer {
	var deployFunc = func(context.Context, string, string) (bool, string, error) {
		return false, "", nil
	}
	return deployer.New(deployFunc, nil, "")
}

var mkNixBuildMock = func(buildOk chan bool) builder.BuildFunc {
	return func(ctx context.Context, drvPath string) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ok := <-buildOk:
			if ok {
				return nil
			} else {
				return fmt.Errorf("An error occured")
			}
		}
	}
}

func TestBuild(t *testing.T) {
	evalOk := make(chan bool)
	buildOk := make(chan bool)
	logrus.SetLevel(logrus.DebugLevel)
	r := utils.NewRepositoryMock()
	f := fetcher.NewFetcher(r)
	tmp := t.TempDir()
	s, _ := store.New(tmp+"/state.json", tmp+"/gcroots", 1, 1)
	f.Start()
	b := builder.New(s, "repoPath", "", "my-machine", 2*time.Second, mkNixEvalMock(evalOk), 2*time.Second, mkNixBuildMock(buildOk))
	var deployFunc = func(context.Context, string, string) (bool, string, error) {
		return false, "profile-path", nil
	}
	d := deployer.New(deployFunc, nil, "")
	e, _ := executor.NewNixOS()
	m := New(s, prometheus.New(), scheduler.New(), f, b, d, "", e)
	go m.Run()
	assert.False(t, m.Fetcher.GetState().IsFetching)
	assert.False(t, m.Builder.State().IsEvaluating)
	assert.False(t, m.Builder.State().IsBuilding)

	commitId := "id-1"
	f.TriggerFetch([]string{"remote"})
	r.RsCh <- repository.RepositoryStatus{
		SelectedCommitId: commitId,
	}
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.True(c, m.Builder.State().IsEvaluating)
		assert.False(c, m.Builder.State().IsBuilding)
	}, 5*time.Second, 100*time.Millisecond)

	// This simulates the failure of an evaluation
	evalOk <- false
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.False(c, m.Builder.State().IsEvaluating)
		assert.False(c, m.Builder.State().IsBuilding)
		g, _ := m.storage.GenerationGet(*m.Builder.GenerationUUID)
		assert.NotNil(c, g.EvalErr)
		assert.Nil(c, m.deployer.GenerationToDeploy)
	}, 5*time.Second, 100*time.Millisecond)

	commitId = "id-2"
	f.TriggerFetch([]string{"remote"})
	r.RsCh <- repository.RepositoryStatus{
		SelectedCommitId: commitId,
	}
	// This simulates the success of an evaluation
	evalOk <- true
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.False(c, m.Builder.State().IsEvaluating)
		assert.True(c, m.Builder.State().IsBuilding)
		g, _ := m.storage.GenerationGet(*m.Builder.GenerationUUID)
		assert.Nil(c, g.EvalErr)
		assert.Nil(c, m.deployer.GenerationToDeploy)
	}, 5*time.Second, 100*time.Millisecond)

	// This simulates the failure of a build
	buildOk <- false
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.False(c, m.Builder.State().IsEvaluating)
		assert.False(c, m.Builder.State().IsBuilding)
		g, _ := m.storage.GenerationGet(*m.Builder.GenerationUUID)
		assert.NotNil(c, g.BuildErr)
		assert.Nil(c, m.deployer.GenerationToDeploy)
	}, 5*time.Second, 100*time.Millisecond)

	// This simulates the success of a build
	f.TriggerFetch([]string{"remote"})
	r.RsCh <- repository.RepositoryStatus{
		SelectedCommitId: "id-3",
	}
	evalOk <- true
	buildOk <- true
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.False(c, m.Builder.State().IsEvaluating)
		assert.False(c, m.Builder.State().IsBuilding)
		g, _ := m.storage.GenerationGet(*m.Builder.GenerationUUID)
		assert.Nil(c, g.BuildErr)
	}, 5*time.Second, 100*time.Millisecond)

	// This simulates the success of another build and ensure this
	// new build is the one proposed for deployment.
	f.TriggerFetch([]string{"remote"})
	r.RsCh <- repository.RepositoryStatus{
		SelectedCommitId: "id-4",
	}
	evalOk <- true
	buildOk <- true
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.False(c, m.Builder.State().IsEvaluating)
		assert.False(c, m.Builder.State().IsBuilding)
		g, _ := m.storage.GenerationGet(*m.Builder.GenerationUUID)
		assert.Nil(c, g.BuildErr)
	}, 5*time.Second, 100*time.Millisecond)

	// This simulates the push of new commit while building
	f.TriggerFetch([]string{"remote"})
	r.RsCh <- repository.RepositoryStatus{
		SelectedCommitId: "id-5",
	}
	evalOk <- true
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.True(c, m.Builder.State().IsBuilding)
	}, 5*time.Second, 100*time.Millisecond)
}

func TestDeploy(t *testing.T) {
	evalOk := make(chan bool)
	buildOk := make(chan bool)
	logrus.SetLevel(logrus.DebugLevel)
	r := utils.NewRepositoryMock()
	f := fetcher.NewFetcher(r)
	f.Start()
	tmp := t.TempDir()
	s, _ := store.New(tmp+"/state.json", tmp+"/gcroots", 1, 1)
	b := builder.New(s, "repoPath", "", "my-machine", 2*time.Second, mkNixEvalMock(evalOk), 2*time.Second, mkNixBuildMock(buildOk))
	var deployFunc = func(context.Context, string, string) (bool, string, error) {
		return false, "profile-path", nil
	}
	d := deployer.New(deployFunc, nil, "")
	e, _ := executor.NewNixOS()
	m := New(s, prometheus.New(), scheduler.New(), f, b, d, "", e)
	go m.Run()
	assert.False(t, m.Fetcher.GetState().IsFetching)
	assert.False(t, m.Builder.State().IsEvaluating)
	assert.False(t, m.Builder.State().IsBuilding)

	m.deployer.Submit(store.Generation{})
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, "profile-path", m.deployer.State().Deployment.ProfilePath)
	}, 5*time.Second, 100*time.Millisecond)

}

func TestIncorrectMachineId(t *testing.T) {
	buildOk := make(chan bool)
	logrus.SetLevel(logrus.DebugLevel)
	r := utils.NewRepositoryMock()
	f := fetcher.NewFetcher(r)
	f.Start()
	nixEval := func(ctx context.Context, repositoryPath string, hostname string) (string, string, string, error) {
		return "drv-path", "out-path", "invalid-machine-id", nil
	}
	tmp := t.TempDir()
	s, _ := store.New(tmp+"/state.json", tmp+"/gcroots", 1, 1)
	b := builder.New(s, "repoPath", "", "my-machine", 2*time.Second, nixEval, 2*time.Second, mkNixBuildMock(buildOk))
	d := mkDeployerMock()
	e, _ := executor.NewNixOS()
	m := New(s, prometheus.New(), scheduler.New(), f, b, d, "the-test-machine-id", e)
	go m.Run()

	f.TriggerFetch([]string{"remote"})
	r.RsCh <- repository.RepositoryStatus{
		SelectedCommitId: "id",
	}

	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.False(t, m.GetState().Builder.IsBuilding)
	}, 5*time.Second, 100*time.Millisecond)
}

func TestCorrectMachineId(t *testing.T) {
	buildOk := make(chan bool)
	logrus.SetLevel(logrus.DebugLevel)
	r := utils.NewRepositoryMock()
	f := fetcher.NewFetcher(r)
	f.Start()
	nixEval := func(ctx context.Context, repositoryPath string, hostname string) (string, string, string, error) {
		return "drv-path", "out-path", "the-test-machine-id", nil
	}
	tmp := t.TempDir()
	s, _ := store.New(tmp+"/state.json", tmp+"/gcroots", 1, 1)
	b := builder.New(s, "repoPath", "", "my-machine", 2*time.Second, nixEval, 2*time.Second, mkNixBuildMock(buildOk))
	d := mkDeployerMock()
	e, _ := executor.NewNixOS()
	m := New(s, prometheus.New(), scheduler.New(), f, b, d, "the-test-machine-id", e)
	go m.Run()

	f.TriggerFetch([]string{"remote"})
	r.RsCh <- repository.RepositoryStatus{
		SelectedCommitId: "id",
	}

	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.True(t, m.GetState().Builder.IsBuilding)
	}, 5*time.Second, 100*time.Millisecond)
}

func TestManagerWithDarwinConfiguration(t *testing.T) {
	r := utils.NewRepositoryMock()
	f := fetcher.NewFetcher(r)
	buildOk := make(chan bool, 1)
	buildOk <- true
	nixEval := func(ctx context.Context, path, hostname string) (drvPath, outPath, machineId string, err error) {
		assert.Equal(t, "my-machine", hostname)
		return "/nix/store/derivation", "/nix/store/outPath", "", nil
	}
	tmp := t.TempDir()
	s, _ := store.New(tmp+"/state.json", tmp+"/gcroots", 1, 1)
	b := builder.New(s, "repoPath", "", "my-machine", 2*time.Second, nixEval, 2*time.Second, mkNixBuildMock(buildOk))
	d := mkDeployerMock()

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
