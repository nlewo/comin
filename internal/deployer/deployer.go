package deployer

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/nlewo/comin/internal/protobuf"
	"github.com/nlewo/comin/internal/store"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type DeployFunc func(context.Context, string, string) (bool, string, error)

type Deployer struct {
	GenerationCh       chan *protobuf.Generation
	deployerFunc       DeployFunc
	DeploymentDoneCh   chan *protobuf.Deployment
	mu                 sync.Mutex
	deployment         atomic.Pointer[protobuf.Deployment]
	previousDeployment atomic.Pointer[protobuf.Deployment]
	isDeploying        atomic.Bool
	// The next generation to deploy. nil when there is no new generation to deploy
	GenerationToDeploy    *protobuf.Generation
	generationAvailableCh chan struct{}
	postDeploymentCommand string

	isSuspended atomic.Bool
	resumeCh    chan struct{}
	// This is true when the runner is actually suspended. This is
	// mainly used for testing purpose.
	runnerIsSuspended atomic.Bool
	store             *store.Store
}

func (d *Deployer) State() *protobuf.Deployer {
	d.mu.Lock()
	defer d.mu.Unlock()
	return &protobuf.Deployer{
		IsDeploying:        wrapperspb.Bool(d.isDeploying.Load()),
		GenerationToDeploy: d.GenerationToDeploy,
		Deployment:         d.deployment.Load(),
		PreviousDeployment: d.previousDeployment.Load(),
		IsSuspended:        wrapperspb.Bool(d.isSuspended.Load()),
	}
}

func (d *Deployer) Deployment() *protobuf.Deployment {
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

func showDeployment(padding string, d *protobuf.Deployment) {
	switch d.Status {
	case store.StatusToString(store.Running):
		fmt.Printf("%sDeployment is running since %s\n", padding, humanize.Time(d.StartedAt.AsTime()))
		fmt.Printf("%sOperation %s\n", padding, d.Operation)
	case store.StatusToString(store.Done):
		fmt.Printf("%sDeployment succeeded %s\n", padding, humanize.Time(d.EndedAt.AsTime()))
		fmt.Printf("%sOperation %s\n", padding, d.Operation)
		fmt.Printf("%sProfilePath %s\n", padding, d.ProfilePath)
	case store.StatusToString(store.Failed):
		fmt.Printf("%sDeployment failed %s\n", padding, humanize.Time(d.EndedAt.AsTime()))
		fmt.Printf("%sOperation %s\n", padding, d.Operation)
		fmt.Printf("%sProfilePath %s\n", padding, d.ProfilePath)
	}
	fmt.Printf("%sGeneration %s\n", padding, d.Generation.Uuid)
	fmt.Printf("%sCommit ID %s from %s/%s\n", padding, d.Generation.SelectedCommitId, d.Generation.SelectedRemoteName, d.Generation.SelectedBranchName)
	fmt.Printf("%sCommit message %s\n", padding, strings.Trim(d.Generation.SelectedCommitMsg, "\n"))
	fmt.Printf("%sOutpath %s\n", padding, d.Generation.OutPath)
}

func Show(s *protobuf.Deployer, padding string) {
	fmt.Printf("  Deployer\n")
	if s.Deployment == nil {
		if s.PreviousDeployment == nil {
			fmt.Printf("%sNo deployment yet\n", padding)
			return
		}
		showDeployment(padding, s.PreviousDeployment)
		return
	}
	showDeployment(padding, s.Deployment)
}

func New(store *store.Store, deployFunc DeployFunc, previousDeployment *protobuf.Deployment, postDeploymentCommand string) *Deployer {
	if previousDeployment != nil {
		logrus.Infof("deployer: initializing with previous deployment %s", previousDeployment.Uuid)
	}
	deployer := &Deployer{
		store:                 store,
		DeploymentDoneCh:      make(chan *protobuf.Deployment, 1),
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
func (d *Deployer) Submit(generation *protobuf.Generation) {
	logrus.Infof("deployer: submiting generation %s", generation.Uuid)
	d.mu.Lock()
	previous := d.previousDeployment.Load()
	if previous == nil || generation.SelectedCommitId != previous.Generation.SelectedCommitId || generation.SelectedBranchIsTesting.GetValue() != previous.Generation.SelectedBranchIsTesting.GetValue() {
		d.GenerationToDeploy = generation
		select {
		case d.generationAvailableCh <- struct{}{}:
		default:
		}
	} else {
		logrus.Infof("deployer: skipping deployment of the generation %s because it is the same than the last deployment", generation.Uuid)
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
			logrus.Infof("deployer: deploying generation %s", g.Uuid)

			operation := "switch"
			if g.SelectedBranchIsTesting.GetValue() {
				operation = "test"
			}
			dpl := d.store.NewDeployment(g, operation)
			d.mu.Lock()
			d.previousDeployment.Swap(d.Deployment())
			d.deployment.Store(dpl)
			d.isDeploying.Store(true)
			d.mu.Unlock()
			if err := d.store.DeploymentStarted(dpl.Uuid); err != nil {
				logrus.Errorf("deployer: could not update the deployment %s in the store", dpl.Uuid)
				continue
			}
			ctx := context.TODO()
			cominNeedRestart, profilePath, err := d.deployerFunc(
				ctx,
				g.OutPath,
				operation,
			)

			deployment := d.Deployment()
			deployment.EndedAt = timestamppb.New(time.Now().UTC())
			if err := d.store.DeploymentFinished(dpl.Uuid, err, cominNeedRestart, profilePath); err != nil {
				logrus.Errorf("deployer: could not update the deployment %s in the store", dpl.Uuid)
				continue
			}
			cmd := d.postDeploymentCommand
			if cmd != "" {
				_, err = runPostDeploymentCommand(cmd, deployment)
				if err != nil {
					logrus.Errorf("deployer: deploying generation %s, post deployment command [%s] failed %v", g.Uuid, cmd, err)
				}
			}

			d.isDeploying.Store(false)
			d.deployment.Store(deployment)
			d.DeploymentDoneCh <- d.Deployment()
		}
	}()
}
