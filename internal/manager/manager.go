// The manager is in charge of managing relationship between
// components. Basically, it receives new commits from the fetcher,
// call the builder to evaluate and build them. Finally, it submits
// these builds to the deployer.

package manager

import (
	"github.com/nlewo/comin/internal/builder"
	"github.com/nlewo/comin/internal/deployer"
	"github.com/nlewo/comin/internal/fetcher"
	"github.com/nlewo/comin/internal/profile"
	"github.com/nlewo/comin/internal/prometheus"
	"github.com/nlewo/comin/internal/scheduler"
	"github.com/nlewo/comin/internal/store"
	"github.com/nlewo/comin/internal/utils"
	"github.com/sirupsen/logrus"
)

type State struct {
	NeedToReboot bool           `json:"need_to_reboot"`
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

	needToReboot            bool
	cominServiceRestartFunc func() error

	prometheus prometheus.Prometheus
	storage    *store.Store
	scheduler  scheduler.Scheduler
	Fetcher    *fetcher.Fetcher
	builder    *builder.Builder
	deployer   *deployer.Deployer
}

func New(s *store.Store, p prometheus.Prometheus, sched scheduler.Scheduler, fetcher *fetcher.Fetcher, builder *builder.Builder, deployer *deployer.Deployer, machineId string) *Manager {
	m := &Manager{
		machineId:               machineId,
		stateRequestCh:          make(chan struct{}),
		stateResultCh:           make(chan State),
		cominServiceRestartFunc: utils.CominServiceRestart,
		prometheus:              p,
		storage:                 s,
		scheduler:               sched,
		Fetcher:                 fetcher,
		builder:                 builder,
		deployer:                deployer,
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
		Fetcher:      m.Fetcher.GetState(),
		Builder:      m.builder.State(),
		Deployer:     m.deployer.State(),
		Store:        m.storage.GetState(),
	}
}

func (m *Manager) Pause() {
}
func (m *Manager) Unpause() {
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
					err := m.builder.Eval(rs)
					if err != nil {
						logrus.Error(err)
					}
				} else {
					logrus.Infof("manager: the commit %s is not evaluated because it is not signed", rs.SelectedCommitId)
				}
			case generationUUID := <-m.builder.EvaluationDone:
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
					logrus.Infof("manager: a generation is building for commit %s", generation.SelectedCommitId)
					_ = m.builder.Build(generationUUID)
				}
			case generationUUID := <-m.builder.BuildDone:
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
	m.needToReboot = utils.NeedToReboot()
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
			m.needToReboot = utils.NeedToReboot()
			m.prometheus.SetHostInfo(m.needToReboot)
			if dpl.RestartComin {
				// TODO: stop contexts
				if err := m.cominServiceRestartFunc(); err != nil {
					logrus.Fatal(err)
					return
				}
			}
		}
	}
}
