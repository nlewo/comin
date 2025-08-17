package deployer

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/google/uuid"
	"github.com/nlewo/comin/internal/store"
	"github.com/sirupsen/logrus"
)

type DeployFunc func(context.Context, string, string) (bool, string, error)

type Deployer struct {
	GenerationCh     chan store.Generation
	deployerFunc     DeployFunc
	DeploymentDoneCh chan store.Deployment
	mu               sync.Mutex
	Deployment       *store.Deployment
	IsDeploying      bool
	// The next generation to deploy. nil when there is no new generation to deploy
	GenerationToDeploy    *store.Generation
	generationAvailableCh chan struct{}
	postDeploymentCommand string

	isSuspended bool
	resumeCh    chan struct{}
	// This is true when the runner is actually suspended. This is
	// mainly used for testing purpose.
	runnerIsSuspended bool
}

type State struct {
	IsDeploying        bool              `json:"is_deploying"`
	GenerationToDeploy *store.Generation `json:"generation_to_deploy"`
	Deployment         *store.Deployment `json:"deployment"`
	IsSuspended        bool              `json:"is_suspended"`
}

func (d *Deployer) State() State {
	return State{
		IsDeploying:        d.IsDeploying,
		GenerationToDeploy: d.GenerationToDeploy,
		Deployment:         d.Deployment,
		IsSuspended:        d.isSuspended,
	}
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
		fmt.Printf("%sNo deployment yet\n", padding)
		return
	}
	showDeployment(padding, *s.Deployment)
}

func New(deployFunc DeployFunc, deployment *store.Deployment, postDeploymentCommand string) *Deployer {
	return &Deployer{
		DeploymentDoneCh:      make(chan store.Deployment, 1),
		deployerFunc:          deployFunc,
		generationAvailableCh: make(chan struct{}, 1),
		Deployment:            deployment,
		postDeploymentCommand: postDeploymentCommand,

		resumeCh: make(chan struct{}, 1),
	}
}

func (d *Deployer) Suspend() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.isSuspended = true
}

func (d *Deployer) Resume() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.isSuspended = false
	select {
	case d.resumeCh <- struct{}{}:
	default:
	}
}

// Retry reties the deployment of the last deployed generation, but
// only when its deployment had failed.
func (d *Deployer) Retry() {
	d.Submit(d.Deployment.Generation)
}

// Submit submits a generation to be deployed. If a deployment is
// running, this generation will be deployed once the current
// deployment is finished. If this generation is the same than the one
// of the last deployment, this generation is skipped.
func (d *Deployer) Submit(generation store.Generation) {
	logrus.Infof("deployer: submiting generation %s", generation.UUID)
	d.mu.Lock()
	if d.Deployment == nil ||
		generation.SelectedCommitId != d.Deployment.Generation.SelectedCommitId ||
		generation.SelectedBranchIsTesting != d.Deployment.Generation.SelectedBranchIsTesting ||
		// This is for the deployer.Retry case
		d.Deployment.Status == store.Failed {

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

			if d.isSuspended {
				d.runnerIsSuspended = true
				<-d.resumeCh
				d.runnerIsSuspended = false
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
			d.Deployment = &dpl
			d.IsDeploying = true
			d.mu.Unlock()

			ctx := context.TODO()
			cominNeedRestart, profilePath, err := d.deployerFunc(
				ctx,
				g.OutPath,
				operation,
			)

			deployment := *(d.Deployment)
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

			d.mu.Lock()
			d.IsDeploying = false
			d.Deployment = &deployment
			d.DeploymentDoneCh <- *d.Deployment
			d.mu.Unlock()

		}
	}()
}
