package deployer

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/google/uuid"
	"github.com/nlewo/comin/internal/store"
	"github.com/sirupsen/logrus"
)

type DeployFunc func(context.Context, string, string) (bool, string, error)

type Deployer struct {
	GenerationCh       chan store.Generation
	deployerFunc       DeployFunc
	DeploymentDoneCh   chan store.Deployment
	mu                 sync.Mutex
	deployment         atomic.Pointer[store.Deployment]
	previousDeployment atomic.Pointer[store.Deployment]
	isDeploying        atomic.Bool
	// The next generation to deploy. nil when there is no new generation to deploy
	GenerationToDeploy    *store.Generation
	generationAvailableCh chan struct{}
	postDeploymentCommand string

	isSuspended atomic.Bool
	resumeCh    chan struct{}
	// This is true when the runner is actually suspended. This is
	// mainly used for testing purpose.
	runnerIsSuspended atomic.Bool
}

type State struct {
	IsDeploying        bool              `json:"is_deploying"`
	GenerationToDeploy *store.Generation `json:"generation_to_deploy"`
	Deployment         *store.Deployment `json:"deployment"`
	PreviousDeployment *store.Deployment `json:"previous_deployment"`
	IsSuspended        bool              `json:"is_suspended"`
}

func (d *Deployer) State() State {
	d.mu.Lock()
	defer d.mu.Unlock()
	return State{
		IsDeploying:        d.isDeploying.Load(),
		GenerationToDeploy: d.GenerationToDeploy,
		Deployment:         d.deployment.Load(),
		PreviousDeployment: d.previousDeployment.Load(),
		IsSuspended:        d.isSuspended.Load(),
	}
}

func (d *Deployer) Deployment() *store.Deployment {
	return d.deployment.Load()
}

func (d *Deployer) IsDeploying() bool {
	return d.isDeploying.Load()
}

func (d *Deployer) RunnerIsSuspended() bool {
	return d.runnerIsSuspended.Load()
}

func (d *Deployer) IsSuspended() bool {
	return d.isSuspended.Load()
}

func showDeployment(padding string, d store.Deployment) {
	switch d.Status {
	case store.Running:
		fmt.Printf("%sDeployment is running since %s\n", padding, humanize.Time(d.StartedAt))
		fmt.Printf("%sOperation %s\n", padding, d.Operation)
	case store.Done:
		fmt.Printf("%sDeployment succeeded %s\n", padding, humanize.Time(d.EndedAt))
		fmt.Printf("%sOperation %s\n", padding, d.Operation)
		fmt.Printf("%sProfilePath %s\n", padding, d.ProfilePath)
	case store.Failed:
		fmt.Printf("%sDeployment failed %s\n", padding, humanize.Time(d.EndedAt))
		fmt.Printf("%sOperation %s\n", padding, d.Operation)
		fmt.Printf("%sProfilePath %s\n", padding, d.ProfilePath)
	}
	fmt.Printf("%sGeneration %s\n", padding, d.Generation.UUID)
	fmt.Printf("%sCommit ID %s from %s/%s\n", padding, d.Generation.SelectedCommitId, d.Generation.SelectedRemoteName, d.Generation.SelectedBranchName)
	fmt.Printf("%sCommit message %s\n", padding, strings.Trim(d.Generation.SelectedCommitMsg, "\n"))
	fmt.Printf("%sOutpath %s\n", padding, d.Generation.OutPath)
}

func (s State) Show(padding string) {
	fmt.Printf("  Deployer\n")
	if s.Deployment == nil {
		if s.PreviousDeployment == nil {
			fmt.Printf("%sNo deployment yet\n", padding)
			return
		}
		showDeployment(padding, *s.PreviousDeployment)
		return
	}
	showDeployment(padding, *s.Deployment)
}

func New(deployFunc DeployFunc, previousDeployment *store.Deployment, postDeploymentCommand string) *Deployer {
	deployer := &Deployer{
		DeploymentDoneCh:      make(chan store.Deployment, 1),
		deployerFunc:          deployFunc,
		generationAvailableCh: make(chan struct{}, 1),
		postDeploymentCommand: postDeploymentCommand,

		resumeCh: make(chan struct{}, 1),
	}

	deployer.previousDeployment.Store(previousDeployment)
	deployer.deployment.Store(previousDeployment)

	return deployer
}

func (d *Deployer) Suspend() {
	d.isSuspended.Store(true)
}

func (d *Deployer) Resume() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.isSuspended.Store(false)
	select {
	case d.resumeCh <- struct{}{}:
	default:
	}
}

// Submit submits a generation to be deployed. If a deployment is
// running, this generation will be deployed once the current
// deployment is finished. If this generation is the same than the one
// of the last deployment, this generation is skipped.
func (d *Deployer) Submit(generation store.Generation) {
	logrus.Infof("deployer: submiting generation %s", generation.UUID)
	d.mu.Lock()
	previous := d.previousDeployment.Load()
	if previous == nil || generation.SelectedCommitId != previous.Generation.SelectedCommitId || generation.SelectedBranchIsTesting != previous.Generation.SelectedBranchIsTesting {
		d.GenerationToDeploy = &generation
		select {
		case d.generationAvailableCh <- struct{}{}:
		default:
		}
	} else {
		logrus.Infof("deployer: skipping deployment of the generation %s because it is the same than the last deployment", generation.UUID)
	}
	d.mu.Unlock()
}

func (d *Deployer) Run() {
	go func() {
		for {
			<-d.generationAvailableCh

			if d.isSuspended.Load() {
				d.runnerIsSuspended.Store(true)
				<-d.resumeCh
				d.runnerIsSuspended.Store(false)
			}

			d.mu.Lock()
			g := d.GenerationToDeploy
			d.GenerationToDeploy = nil
			d.mu.Unlock()
			logrus.Infof("deployer: deploying generation %s", g.UUID)

			operation := "switch"
			if g.SelectedBranchIsTesting {
				operation = "test"
			}
			dpl := store.Deployment{
				UUID:       uuid.NewString(),
				Generation: *g,
				Operation:  operation,
				StartedAt:  time.Now().UTC(),
				Status:     store.Running,
			}
			d.mu.Lock()
			d.previousDeployment.Swap(d.Deployment())
			d.deployment.Store(&dpl)
			d.isDeploying.Store(true)
			d.mu.Unlock()

			ctx := context.TODO()
			cominNeedRestart, profilePath, err := d.deployerFunc(
				ctx,
				g.OutPath,
				operation,
			)

			deployment := *d.Deployment()
			deployment.EndedAt = time.Now().UTC()
			deployment.Err = err
			if err != nil {
				deployment.ErrorMsg = err.Error()
				deployment.Status = store.Failed
			} else {
				deployment.Status = store.Done
			}
			deployment.RestartComin = cominNeedRestart
			deployment.ProfilePath = profilePath

			cmd := d.postDeploymentCommand
			if cmd != "" {
				_, err = runPostDeploymentCommand(cmd, deployment)
				if err != nil {
					logrus.Errorf("deployer: deploying generation %s, post deployment command [%s] failed %v", g.UUID, cmd, err)
				}
			}

			d.isDeploying.Store(false)
			d.deployment.Store(&deployment)
			d.DeploymentDoneCh <- *d.Deployment()
		}
	}()
}
