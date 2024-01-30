package manager

import (
	"context"
	"time"

	"github.com/nlewo/comin/internal/deployment"
	"github.com/nlewo/comin/internal/generation"
	"github.com/nlewo/comin/internal/nix"
	"github.com/nlewo/comin/internal/repository"
	"github.com/nlewo/comin/internal/utils"
	"github.com/sirupsen/logrus"
)

type State struct {
	RepositoryStatus repository.RepositoryStatus `json:"repository_status"`
	generation       generation.Generation
	IsFetching       bool                  `json:"is_fetching"`
	IsRunning        bool                  `json:"is_running"`
	Deployment       deployment.Deployment `json:"deployment"`
	Hostname         string                `json:"hostname"`
}

type Manager struct {
	repository repository.Repository
	// FIXME: a generation should get a repository URL from the repository status
	repositoryPath string
	hostname       string
	// The machine id of the current host
	machineId         string
	triggerRepository chan string
	generationFactory func(repository.RepositoryStatus, string, string) generation.Generation
	stateRequestCh    chan struct{}
	stateResultCh     chan State
	repositoryStatus  repository.RepositoryStatus
	// The generation currently managed
	generation generation.Generation
	isFetching bool
	// FIXME: this is temporary in order to simplify the manager
	// for a first iteration: this needs to be removed
	isRunning               bool
	needToBeRestarted       bool
	cominServiceRestartFunc func() error

	evalFunc  generation.EvalFunc
	buildFunc generation.BuildFunc

	deploymentResultCh chan deployment.DeploymentResult
	// The deployment currenly managed
	deployment   deployment.Deployment
	deployerFunc deployment.DeployFunc
}

func New(r repository.Repository, path, hostname, machineId string) Manager {
	return Manager{
		repository:              r,
		repositoryPath:          path,
		hostname:                hostname,
		machineId:               machineId,
		evalFunc:                nix.Eval,
		buildFunc:               nix.Build,
		deployerFunc:            nix.Deploy,
		triggerRepository:       make(chan string),
		stateRequestCh:          make(chan struct{}),
		stateResultCh:           make(chan State),
		cominServiceRestartFunc: utils.CominServiceRestart,
		deploymentResultCh:      make(chan deployment.DeploymentResult),
	}
}

func (m Manager) GetState() State {
	m.stateRequestCh <- struct{}{}
	return <-m.stateResultCh
}

func (m Manager) Fetch(remote string) {
	m.triggerRepository <- remote
}

func (m Manager) toState() State {
	return State{
		generation:       m.generation,
		RepositoryStatus: m.repositoryStatus,
		IsFetching:       m.isFetching,
		IsRunning:        m.isRunning,
		Deployment:       m.deployment,
		Hostname:         m.hostname,
	}
}

func (m Manager) Run() {
	var repositoryStatusCh chan repository.RepositoryStatus
	var evalResultCh chan generation.EvalResult
	var buildResultCh chan generation.BuildResult
	ctx := context.TODO()

	logrus.Info("The manager is started")
	logrus.Infof("  hostname = %s", m.hostname)
	logrus.Infof("  machineId = %s", m.machineId)
	logrus.Infof("  repositoryPath = %s", m.repositoryPath)
	for {
		select {
		case <-m.stateRequestCh:
			m.stateResultCh <- m.toState()
		case remoteName := <-m.triggerRepository:
			if m.isFetching {
				logrus.Debugf("The manager is already fetching the repository")
				continue
			}
			// FIXME: we will remove this in future versions
			if m.isRunning {
				logrus.Debugf("The manager is already running: it is currently not able to run tasks in parallel")
				continue
			}
			logrus.Debugf("Trigger fetch and update remote %s", remoteName)
			m.isRunning = true
			m.isFetching = true
			repositoryStatusCh = m.repository.FetchAndUpdate(ctx, remoteName)
		case rs := <-repositoryStatusCh:
			logrus.Debugf("Fetch done with %#v", rs)
			m.isFetching = false
			m.repositoryStatus = rs
			if rs.Equal(m.generation.RepositoryStatus) {
				logrus.Debugf("The repository status is the same than the previous one")
				m.isRunning = false
			} else {
				// g.Stop(): this is required once we remove m.IsRunning
				m.generation = generation.New(rs, m.repositoryPath, m.hostname, m.machineId, m.evalFunc, m.buildFunc)
				m.generation.EvalStartedAt = time.Now()
				// FIXME: we need to let nix fetching a git commit from the repository instead of using the repository
				// directory which an be updated in parallel
				evalResultCh = m.generation.Eval(ctx)
			}
		case evalResult := <-evalResultCh:
			logrus.Debugf("Eval done with %#v", evalResult)
			m.generation.EvalResult = evalResult
			if evalResult.Err == nil {
				m.generation.BuildStartedAt = time.Now()
				buildResultCh = m.generation.Build(ctx)
			} else {
				m.isRunning = false
			}
		case buildResult := <-buildResultCh:
			logrus.Debugf("Build done with %#v", buildResult)
			m.generation.BuildResult = buildResult
			if buildResult.Err == nil {
				m.deployment = deployment.New(m.generation, m.deployerFunc, m.deploymentResultCh)
				m.deployment = m.deployment.Deploy(ctx)
			} else {
				m.isRunning = false
			}
		case deploymentResult := <-m.deploymentResultCh:
			logrus.Debugf("Deploy done with %#v", deploymentResult)
			m.deployment = m.deployment.Update(deploymentResult)
			// The comin service is not restart by the switch-to-configuration script in order to let comin terminating properly. Instead, comin restarts itself.
			if m.deployment.RestartComin {
				m.needToBeRestarted = true
				break
			}
			m.isRunning = false
		}

		if m.needToBeRestarted {
			// TODO: stop contexts
			if err := m.cominServiceRestartFunc(); err != nil {
				logrus.Fatal(err)
				return
			}

		}
	}
}
