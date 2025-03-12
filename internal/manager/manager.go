package manager

import (
	"fmt"

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
	NeedToReboot bool `json:"need_to_reboot"`
	// This is set if a user confirmation is asked before
	// building (and deploying) a generation
	ConfirmBuildEnabled  bool           `json:"confirm_build_enabled"`
	ConfirmBuildRequired bool           `json:"confirm_build_required"`
	Fetcher              fetcher.State  `json:"fetcher"`
	Builder              builder.State  `json:"builder"`
	Deployer             deployer.State `json:"deployer"`
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
	storage    store.Store
	scheduler  scheduler.Scheduler
	Fetcher    *fetcher.Fetcher
	builder    *builder.Builder
	deployer   *deployer.Deployer

	confirmBuildEnabled   bool
	confirmBuildRequired  bool
	confirmBuildCh        chan string
	generationUUIDToBuild string
}

func New(s store.Store, p prometheus.Prometheus, sched scheduler.Scheduler, fetcher *fetcher.Fetcher, builder *builder.Builder, deployer *deployer.Deployer, machineId string, confirmBuild bool) *Manager {
	m := &Manager{
		machineId:               machineId,
		confirmBuildEnabled:     confirmBuild,
		confirmBuildCh:          make(chan string),
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
		NeedToReboot:         m.needToReboot,
		ConfirmBuildEnabled:  m.confirmBuildEnabled,
		ConfirmBuildRequired: m.confirmBuildRequired,
		Fetcher:              m.Fetcher.GetState(),
		Builder:              m.builder.State(),
		Deployer:             m.deployer.State(),
	}
}

func (m *Manager) ConfirmBuild(generationUUID string) error {
	if !m.confirmBuildEnabled {
		return fmt.Errorf("Build doesn't need to be confirm to get deployed")
	}
	if !m.confirmBuildRequired {
		return fmt.Errorf("No build confirmation is required")
	}
	if gUUID := m.builder.State().Generation.UUID; gUUID != generationUUID {
		return fmt.Errorf("The current builder.generation.UUID %s is not equal to the requested generation UUID %s", gUUID, generationUUID)
	}
	m.confirmBuildCh <- generationUUID
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
					m.builder.Eval(rs)
				} else {
					logrus.Infof("manager: the commit %s is not evaluated because it is not signed", rs.SelectedCommitId)
				}
			case generation := <-m.builder.EvaluationDone:
				if generation.EvalErr != nil {
					continue
				}
				if generation.MachineId != "" && m.machineId != generation.MachineId {
					logrus.Infof("manager: the comin.machineId %s is not the host machine-id %s", generation.MachineId, m.machineId)
				} else {
					if m.confirmBuildEnabled {
						logrus.Infof("manager: a build confirmation is required")
						m.confirmBuildRequired = true
					} else {
						if err := m.builder.Build(generation.UUID); err != nil {
							logrus.Error(err)
						}
					}
				}
			case generationUUID := <-m.confirmBuildCh:
				m.confirmBuildRequired = false
				if err := m.builder.Build(generationUUID); err != nil {
					logrus.Error(err)
				}
			case generation := <-m.builder.BuildDone:
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
			m.prometheus.SetDeploymentInfo(dpl.Generation.SelectedCommitId, deployer.StatusToString(dpl.Status))
			getsEvicted, evicted := m.storage.DeploymentInsertAndCommit(dpl)
			if getsEvicted && evicted.ProfilePath != "" {
				profile.RemoveProfilePath(evicted.ProfilePath)
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
