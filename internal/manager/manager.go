// The manager is in charge of managing relationship between
// components. Basically, it receives new commits from the fetcher,
// call the builder to evaluate and build them. Finally, it submits
// these builds to the deployer.

package manager

import (
	"fmt"
	"os"

	"github.com/nlewo/comin/internal/builder"
	"github.com/nlewo/comin/internal/deployer"
	"github.com/nlewo/comin/internal/executor"
	"github.com/nlewo/comin/internal/fetcher"
	"github.com/nlewo/comin/internal/profile"
	"github.com/nlewo/comin/internal/prometheus"
	"github.com/nlewo/comin/internal/scheduler"
	"github.com/nlewo/comin/internal/store"
	"github.com/sirupsen/logrus"
)

type State struct {
	NeedToReboot bool           `json:"need_to_reboot"`
	IsSuspended  bool           `json:"suspended"`
	Fetcher      fetcher.State  `json:"fetcher"`
	Builder      builder.State  `json:"builder"`
	Deployer     deployer.State `json:"deployer"`
	Store        store.State    `json:"store"`
}

type Manager struct {
	// The machine id of the current host. It is used to ensure
	// the optionnal machine-id found at evaluation time
	// corresponds to the machine-id of this host.
	machineId string

	stateRequestCh chan struct{}
	stateResultCh  chan State

	needToReboot bool

	prometheus prometheus.Prometheus
	storage    *store.Store
	scheduler  scheduler.Scheduler
	Fetcher    *fetcher.Fetcher
	Builder    *builder.Builder
	deployer   *deployer.Deployer
	executor   executor.Executor

	isSuspended bool
}

func New(s *store.Store, p prometheus.Prometheus, sched scheduler.Scheduler, fetcher *fetcher.Fetcher, builder *builder.Builder, deployer *deployer.Deployer, machineId string, executor executor.Executor) *Manager {
	m := &Manager{
		machineId:      machineId,
		stateRequestCh: make(chan struct{}),
		stateResultCh:  make(chan State),
		prometheus:     p,
		storage:        s,
		scheduler:      sched,
		Fetcher:        fetcher,
		Builder:        builder,
		deployer:       deployer,
		executor:       executor,
	}
	return m
}

func (m *Manager) GetState() State {
	m.stateRequestCh <- struct{}{}
	return <-m.stateResultCh
}

func (m *Manager) toState() State {
	return State{
		NeedToReboot: m.needToReboot,
		IsSuspended:  m.isSuspended,
		Fetcher:      m.Fetcher.GetState(),
		Builder:      m.Builder.State(),
		Deployer:     m.deployer.State(),
		Store:        m.storage.GetState(),
	}
}

func (m *Manager) Suspend() error {
	if m.isSuspended {
		return fmt.Errorf("the manager is already suspended")
	}
	if err := m.Builder.Suspend(); err != nil {
		return err
	}
	m.deployer.Suspend()
	m.isSuspended = true
	return nil
}

func (m *Manager) Resume() error {
	if !m.isSuspended {
		return fmt.Errorf("the manager is not suspended")
	}
	if err := m.Builder.Resume(); err != nil {
		return err
	}
	m.deployer.Resume()
	m.isSuspended = false
	return nil
}

// FetchAndBuild fetches new commits. If a new commit is available, it
// evaluates and builds the derivation. Once built, it pushes the
// generation on a channel which is consumed by the deployer.
func (m *Manager) FetchAndBuild() {
	go func() {
		for {
			select {
			case rs := <-m.Fetcher.RepositoryStatusCh:
				if !rs.SelectedCommitShouldBeSigned || rs.SelectedCommitSigned {
					logrus.Infof("manager: a generation is evaluating for commit %s", rs.SelectedCommitId)
					err := m.Builder.Eval(rs)
					if err != nil {
						logrus.Error(err)
					}
				} else {
					logrus.Infof("manager: the commit %s is not evaluated because it is not signed", rs.SelectedCommitId)
				}
			case generationUUID := <-m.Builder.EvaluationDone:
				generation, err := m.storage.GenerationGet(generationUUID)
				if err != nil {
					logrus.Error(err)
					continue
				}
				if generation.EvalErr != nil {
					continue
				}
				if generation.MachineId != "" && m.machineId != generation.MachineId {
					logrus.Infof("manager: the comin.machineId %s is not the host machine-id %s", generation.MachineId, m.machineId)
				} else {
					logrus.Infof("manager: the build of the generation %s is submitted", generation.UUID.String())
					m.Builder.SubmitBuild(generationUUID)
				}
			case generationUUID := <-m.Builder.BuildDone:
				generation, err := m.storage.GenerationGet(generationUUID)
				if err != nil {
					logrus.Error(err)
					continue
				}
				if generation.BuildErr == nil {
					logrus.Infof("manager: a generation is available for deployment with commit %s", generation.SelectedCommitId)
					m.deployer.Submit(generation)
				}
			}
		}

	}()
}

func (m *Manager) Run() {
	logrus.Infof("manager: starting with machineId=%s", m.machineId)
	m.needToReboot = m.executor.NeedToReboot()
	m.prometheus.SetHostInfo(m.needToReboot)

	m.FetchAndBuild()
	m.deployer.Run()

	for {
		select {
		case <-m.stateRequestCh:
			m.stateResultCh <- m.toState()
		case dpl := <-m.deployer.DeploymentDoneCh:
			m.prometheus.SetDeploymentInfo(dpl.Generation.SelectedCommitId, store.StatusToString(dpl.Status))
			getsEvicted, evicted := m.storage.DeploymentInsertAndCommit(dpl)
			if getsEvicted && evicted.ProfilePath != "" {
				_ = profile.RemoveProfilePath(evicted.ProfilePath)
			}
			m.needToReboot = m.executor.NeedToReboot()
			m.prometheus.SetHostInfo(m.needToReboot)
			if dpl.RestartComin {
				// TODO: stop contexts
				logrus.Infof("manager: comin needs to be restarted")
				logrus.Infof("manager: exiting comin to let the service manager restart it")
				os.Exit(0)
			}
		}
	}
}
