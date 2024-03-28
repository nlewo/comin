package manager

import (
	"context"
	"testing"
	"time"

	"github.com/nlewo/comin/internal/deployment"
	"github.com/nlewo/comin/internal/prometheus"
	"github.com/nlewo/comin/internal/repository"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

type metricsMock struct{}

func (m metricsMock) SetDeploymentInfo(commitId, status string) {}

type repositoryMock struct {
	rsCh chan repository.RepositoryStatus
}

func newRepositoryMock() (r *repositoryMock) {
	rsCh := make(chan repository.RepositoryStatus)
	return &repositoryMock{
		rsCh: rsCh,
	}
}
func (r *repositoryMock) FetchAndUpdate(ctx context.Context, remoteName string) (rsCh chan repository.RepositoryStatus) {
	return r.rsCh
}

func TestRun(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	r := newRepositoryMock()
	m := New(r, prometheus.New(), "", "", "")

	evalDone := make(chan struct{})
	buildDone := make(chan struct{})
	nixEvalMock := func(ctx context.Context, repositoryPath string, hostname string) (string, string, string, error) {
		<-evalDone
		return "drv-path", "out-path", "", nil
	}
	nixBuildMock := func(ctx context.Context, drvPath string) error {
		<-buildDone
		return nil
	}
	m.evalFunc = nixEvalMock
	m.buildFunc = nixBuildMock

	deployFunc := func(context.Context, string, string, string) (bool, error) {
		return false, nil
	}
	m.deployerFunc = deployFunc

	go m.Run()

	// the state is empty
	assert.Equal(t, State{}, m.GetState())

	// the repository is fetched
	m.Fetch("origin")
	assert.Equal(t, repository.RepositoryStatus{}, m.GetState().RepositoryStatus)

	// we inject a repositoryStatus
	r.rsCh <- repository.RepositoryStatus{
		SelectedCommitId: "foo",
	}
	assert.Equal(
		t,
		repository.RepositoryStatus{SelectedCommitId: "foo"},
		m.GetState().RepositoryStatus)

	// we simulate the end of the evaluation
	close(evalDone)
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, "drv-path", m.GetState().Generation.DrvPath)
		assert.NotEmpty(c, m.GetState().Generation.EvalEndedAt)
	}, 5*time.Second, 100*time.Millisecond, "evaluation is not finished")

	// we simulate the end of the build
	close(buildDone)
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.NotEmpty(c, m.GetState().Generation.BuildEndedAt)
	}, 5*time.Second, 100*time.Millisecond, "build is not finished")

	// we simulate the end of the deploy
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.NotEmpty(c, m.GetState().Deployment.EndAt)
	}, 5*time.Second, 100*time.Millisecond, "deployment is not finished")

}

func TestFetchBusy(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	r := newRepositoryMock()
	m := New(r, prometheus.New(), "", "", "machine-id")
	go m.Run()

	assert.Equal(t, State{}, m.GetState())

	m.Fetch("origin")
	assert.Equal(t, repository.RepositoryStatus{}, m.GetState().RepositoryStatus)

	m.Fetch("origin")
	assert.Equal(t, repository.RepositoryStatus{}, m.GetState().RepositoryStatus)
}

func TestRestartComin(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	r := newRepositoryMock()
	m := New(r, prometheus.New(), "", "", "machine-id")
	dCh := make(chan deployment.DeploymentResult)
	m.deploymentResultCh = dCh
	isCominRestarted := false
	cominServiceRestartMock := func() error {
		isCominRestarted = true
		return nil
	}
	m.cominServiceRestartFunc = cominServiceRestartMock
	go m.Run()
	m.deploymentResultCh <- deployment.DeploymentResult{
		RestartComin: true,
	}
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.True(c, isCominRestarted)
	}, 5*time.Second, 100*time.Millisecond, "comin has not been restarted yet")

}

func TestOptionnalMachineId(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	r := newRepositoryMock()
	m := New(r, prometheus.New(), "", "", "the-test-machine-id")

	evalDone := make(chan struct{})
	buildDone := make(chan struct{})
	nixEvalMock := func(ctx context.Context, repositoryPath string, hostname string) (string, string, string, error) {
		<-evalDone
		// When comin.machineId is empty, comin evaluates it as an empty string
		evaluatedMachineId := ""
		return "drv-path", "out-path", evaluatedMachineId, nil
	}
	nixBuildMock := func(ctx context.Context, drvPath string) error {
		<-buildDone
		return nil
	}
	m.evalFunc = nixEvalMock
	m.buildFunc = nixBuildMock

	go m.Run()
	m.Fetch("origin")
	r.rsCh <- repository.RepositoryStatus{SelectedCommitId: "foo"}

	// we simulate the end of the evaluation
	close(evalDone)

	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.True(t, m.GetState().IsRunning)
	}, 5*time.Second, 100*time.Millisecond, "evaluation is not finished")
}

func TestIncorrectMachineId(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	r := newRepositoryMock()
	m := New(r, prometheus.New(), "", "", "the-test-machine-id")

	evalDone := make(chan struct{})
	buildDone := make(chan struct{})
	nixEvalMock := func(ctx context.Context, repositoryPath string, hostname string) (string, string, string, error) {
		<-evalDone
		return "drv-path", "out-path", "incorrect-machine-id", nil
	}
	nixBuildMock := func(ctx context.Context, drvPath string) error {
		<-buildDone
		return nil
	}
	m.evalFunc = nixEvalMock
	m.buildFunc = nixBuildMock

	go m.Run()

	// the state is empty
	assert.Equal(t, State{}, m.GetState())

	// the repository is fetched
	m.Fetch("origin")
	r.rsCh <- repository.RepositoryStatus{SelectedCommitId: "foo"}

	assert.True(t, m.GetState().IsRunning)

	// we simulate the end of the evaluation
	close(evalDone)

	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		// The manager is no longer running since the machine id are not identical
		assert.False(t, m.GetState().IsRunning)
	}, 5*time.Second, 100*time.Millisecond, "evaluation is not finished")
}
