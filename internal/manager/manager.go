// The manager is in charge of managing relationship between
// components. Basically, it receives new commits from the fetcher,
// call the builder to evaluate and build them. Finally, it submits
// these builds to the deployer.

package manager

import (
	"context"
	"fmt"
	"os"

	"github.com/nlewo/comin/internal/broker"
	"github.com/nlewo/comin/internal/builder"
	"github.com/nlewo/comin/internal/deployer"
	"github.com/nlewo/comin/internal/executor"
	"github.com/nlewo/comin/internal/fetcher"
	"github.com/nlewo/comin/internal/profile"
	"github.com/nlewo/comin/internal/prometheus"
	"github.com/nlewo/comin/internal/protobuf"
	"github.com/nlewo/comin/internal/scheduler"
	"github.com/nlewo/comin/internal/store"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type Manager struct {
	// The machine id of the current host. It is used to ensure
	// the optionnal machine-id found at evaluation time
	// corresponds to the machine-id of this host.
	machineId string

	stateRequestCh chan struct{}
	stateResultCh  chan *protobuf.State

	needToReboot bool

	prometheus      prometheus.Prometheus
	storage         *store.Store
	scheduler       scheduler.Scheduler
	Fetcher         *fetcher.Fetcher
	Builder         *builder.Builder
	deployer        *deployer.Deployer
	executor        executor.Executor
	BuildConfirmer  *Confirmer
	DeployConfirmer *Confirmer

	configurationOperations ConfigurationOperations

	isSuspended bool

	broker *broker.Broker
}

func New(s *store.Store,
	p prometheus.Prometheus,
	sched scheduler.Scheduler,
	fetcher *fetcher.Fetcher,
	builder *builder.Builder,
	deployer *deployer.Deployer,
	machineId string,
	executor executor.Executor,
	buildConfirmer *Confirmer,
	deployConfirmer *Confirmer,
	broker *broker.Broker,
	configurationOperations ConfigurationOperations,
) *Manager {

	m := &Manager{
		machineId:               machineId,
		stateRequestCh:          make(chan struct{}),
		stateResultCh:           make(chan *protobuf.State),
		prometheus:              p,
		storage:                 s,
		scheduler:               sched,
		Fetcher:                 fetcher,
		Builder:                 builder,
		deployer:                deployer,
		executor:                executor,
		BuildConfirmer:          buildConfirmer,
		DeployConfirmer:         deployConfirmer,
		broker:                  broker,
		configurationOperations: configurationOperations,
	}
	return m
}

func (m *Manager) GetState() *protobuf.State {
	m.stateRequestCh <- struct{}{}
	return <-m.stateResultCh
}

func (m *Manager) toState() *protobuf.State {
	return &protobuf.State{
		NeedToReboot:    wrapperspb.Bool(m.needToReboot),
		IsSuspended:     wrapperspb.Bool(m.isSuspended),
		Builder:         m.Builder.State(),
		Deployer:        m.deployer.State(),
		Fetcher:         m.Fetcher.GetState(),
		Store:           m.storage.GetState(),
		BuildConfirmer:  m.BuildConfirmer.status(),
		DeployConfirmer: m.DeployConfirmer.status(),
	}
}

func (m *Manager) SwitchDeploymentLatest() error {
	latest := m.storage.GetDeploymentLastest()
	if latest == nil {
		return fmt.Errorf("manager: no previous deployment")
	}
	m.deployer.Submit(latest.Generation, "switch")
	return nil
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
	m.prometheus.SetHostInfo(m.needToReboot, m.isSuspended)
	m.prometheus.SetIsSuspended(m.isSuspended)
	m.broker.Publish(&protobuf.Event{Type: &protobuf.Event_Suspend_{Suspend: &protobuf.Event_Suspend{}}})
	return nil
}

func (m *Manager) Resume(ctx context.Context) error {
	if !m.isSuspended {
		return fmt.Errorf("the manager is not suspended")
	}
	if err := m.Builder.Resume(ctx); err != nil {
		return err
	}
	m.deployer.Resume()
	m.isSuspended = false
	m.prometheus.SetHostInfo(m.needToReboot, m.isSuspended)
	m.prometheus.SetIsSuspended(m.isSuspended)
	m.broker.Publish(&protobuf.Event{Type: &protobuf.Event_Resume_{Resume: &protobuf.Event_Resume{}}})
	return nil
}

// FetchAndBuild fetches new commits. If a new commit is available, it
// evaluates and builds the derivation. Once built, it pushes the
// generation on a channel which is consumed by the deployer.
func (m *Manager) FetchAndBuild(ctx context.Context) {
	go func() {
		for {
			select {
			case rs := <-m.Fetcher.RepositoryStatusCh:
				if !rs.SelectedCommitShouldBeSigned.GetValue() || rs.SelectedCommitSigned.GetValue() {
					logrus.Infof("manager: a generation is evaluating for commit %s", rs.SelectedCommitId)
					err := m.Builder.Eval(ctx, rs)
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
				if generation.EvalErr != "" {
					continue
				}
				if generation.MachineId != "" && m.machineId != generation.MachineId {
					logrus.Infof("manager: the comin.machineId %s is not the host machine-id %s", generation.MachineId, m.machineId)
				} else {
					logrus.Infof("manager: the build of the generation %s is submitted", generation.Uuid)
					m.BuildConfirmer.Submit(generationUUID)
				}
			case generationUUID := <-m.BuildConfirmer.confirmed:
				m.Builder.SubmitBuild(ctx, generationUUID)

			case generationUUID := <-m.Builder.BuildDone:
				generation, err := m.storage.GenerationGet(generationUUID)
				if err != nil {
					logrus.Error(err)
					continue
				}
				if generation.BuildErr == "" {
					logrus.Infof("manager: a generation is available for deployment with commit %s", generation.SelectedCommitId)
					if !m.deployer.IsAlreadyDeployed(&generation) {
						m.DeployConfirmer.Submit(generationUUID)
					}
				}
			case generationUUID := <-m.DeployConfirmer.confirmed:
				generation, err := m.storage.GenerationGet(generationUUID)
				if err != nil {
					logrus.Error(err)
					continue
				}
				operation := m.getOperationFromConfigurationOperations(generation.SelectedRemoteName, generation.SelectedBranchName)
				m.deployer.Submit(&generation, operation)
			}
		}
	}()
}

// ConfigurationOperations is a map describing the operation associated
// to each remote/branch. It is a map looking such as:
// { origin: { main: switch, testing: test }, local { main: switch }
type ConfigurationOperations map[string](map[string]string)

func (m *Manager) getOperationFromConfigurationOperations(remote, branch string) (operation string) {
	operation = "test"
	branches, ok := m.configurationOperations[remote]
	if !ok {
		logrus.Errorf("manager: could not get the remote %s. Assuming 'test' operation", remote)
		return
	}
	operation, ok = branches[branch]
	if !ok {
		logrus.Errorf("manager: could not get the operation for the branch %s/%s. Assuming test operation", remote, branch)
		return
	}
	return
}

func (m *Manager) Run(ctx context.Context) {
	logrus.Infof("manager: starting with machineId=%s", m.machineId)
	lastDpl := m.deployer.State().Deployment
	if lastDpl != nil {
		m.needToReboot = m.executor.NeedToReboot(lastDpl.Generation.OutPath, lastDpl.Operation)
	}
	m.prometheus.SetHostInfo(m.needToReboot, m.isSuspended)
	m.prometheus.SetNeedToReboot(m.needToReboot)
	m.prometheus.SetIsSuspended(m.isSuspended)

	m.FetchAndBuild(ctx)
	m.deployer.Run(ctx)

	for {
		select {
		case <-m.stateRequestCh:
			m.stateResultCh <- m.toState()
		case dpl := <-m.deployer.DeploymentDoneCh:
			m.prometheus.SetDeploymentInfo(dpl.Generation.SelectedCommitId, dpl.Status)
			getsEvicted, evicted := m.storage.DeploymentInsertAndCommit(dpl)

			// We remove the evicted deployment profile
			// path only if this profile path is not used
			// by any still alive other deployments.
			if getsEvicted && evicted.ProfilePath != "" {
				alive := false
				for _, d := range m.storage.DeploymentList() {
					if d.ProfilePath == evicted.ProfilePath {
						alive = true
					}
				}
				if !alive {
					_ = profile.RemoveProfilePath(evicted.ProfilePath)
				}
			}
			m.needToReboot = m.executor.NeedToReboot(dpl.Generation.OutPath, dpl.Operation)
			if m.needToReboot {
				e := &protobuf.Event_RebootRequired{Deployment: dpl}
				m.broker.Publish(&protobuf.Event{Type: &protobuf.Event_RebootRequired_{RebootRequired: e}})
			}
			m.prometheus.SetHostInfo(m.needToReboot, m.isSuspended)
			m.prometheus.SetNeedToReboot(m.needToReboot)
			if dpl.RestartComin.GetValue() {
				// TODO: stop contexts
				logrus.Infof("manager: comin needs to be restarted")
				logrus.Infof("manager: exiting comin to let the service manager restart it")
				os.Exit(0)
			}
		}
	}
}
