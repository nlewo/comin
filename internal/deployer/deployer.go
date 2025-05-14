package deployer

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/google/uuid"
	"github.com/nlewo/comin/internal/builder"
	"github.com/sirupsen/logrus"
)

type Status int64

const (
	Init Status = iota
	Running
	Done
	Failed
)

func StatusToString(status Status) string {
	switch status {
	case Init:
		return "init"
	case Running:
		return "running"
	case Done:
		return "done"
	case Failed:
		return "failed"
	}
	return ""
}

type Deployment struct {
	UUID       string             `json:"uuid"`
	Generation builder.Generation `json:"generation"`
	StartedAt  time.Time          `json:"started_at"`
	EndedAt    time.Time          `json:"ended_at"`
	// It is ignored in the JSON marshaling
	Err          error  `json:"-"`
	ErrorMsg     string `json:"error_msg"`
	RestartComin bool   `json:"restart_comin"`
	ProfilePath  string `json:"profile_path"`
	Status       Status `json:"status"`
	Operation    string `json:"operation"`
}

type DeployFunc func(context.Context, string, string) (bool, string, error)

type Deployer struct {
	GenerationCh       chan builder.Generation
	deployerFunc       DeployFunc
	DeploymentDoneCh   chan Deployment
	mu                 sync.Mutex
	Deployment         *Deployment
	previousDeployment *Deployment
	IsDeploying        bool
	// The next generation to deploy. nil when there is no new generation to deploy
	GenerationToDeploy    *builder.Generation
	generationAvailableCh chan struct{}
	postDeploymentCommand string
}

func (d Deployment) IsTesting() bool {
	return d.Operation == "test"
}

type State struct {
	IsDeploying        bool                `json:"is_deploying"`
	GenerationToDeploy *builder.Generation `json:"generation_to_deploy"`
	Deployment         *Deployment         `json:"deployment"`
	PreviousDeployment *Deployment         `json:"previous_deployment"`
}

func (d *Deployer) State() State {
	return State{
		IsDeploying:        d.IsDeploying,
		GenerationToDeploy: d.GenerationToDeploy,
		Deployment:         d.Deployment,
		PreviousDeployment: d.previousDeployment,
	}
}

func showDeployment(padding string, d Deployment) {
	switch d.Status {
	case Running:
		fmt.Printf("%sDeployment is running since %s\n", padding, humanize.Time(d.StartedAt))
		fmt.Printf("%sOperation %s\n", padding, d.Operation)
	case Done:
		fmt.Printf("%sDeployment succeeded %s\n", padding, humanize.Time(d.EndedAt))
		fmt.Printf("%sOperation %s\n", padding, d.Operation)
		fmt.Printf("%sProfilePath %s\n", padding, d.ProfilePath)
	case Failed:
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
		showDeployment(padding, *s.PreviousDeployment)
		return
	}
	showDeployment(padding, *s.Deployment)
}

func New(deployFunc DeployFunc, previousDeployment *Deployment, postDeploymentCommand string) *Deployer {
	return &Deployer{
		DeploymentDoneCh:      make(chan Deployment, 1),
		deployerFunc:          deployFunc,
		generationAvailableCh: make(chan struct{}, 1),
		previousDeployment:    previousDeployment,
		Deployment:            previousDeployment,
		postDeploymentCommand: postDeploymentCommand,
	}
}

// Submit submits a generation to be deployed. If a deployment is
// running, this generation will be deployed once the current
// deployment is finished. If this generation is the same than the one
// of the last deployment, this generation is skipped.
func (d *Deployer) Submit(generation builder.Generation) {
	logrus.Infof("deployer: submiting generation %s", generation.UUID)
	d.mu.Lock()
	if d.previousDeployment == nil || generation.SelectedCommitId != d.previousDeployment.Generation.SelectedCommitId || generation.SelectedBranchIsTesting != d.previousDeployment.Generation.SelectedBranchIsTesting {
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
			d.mu.Lock()
			g := d.GenerationToDeploy
			d.GenerationToDeploy = nil
			d.mu.Unlock()
			logrus.Infof("deployer: deploying generation %s", g.UUID)

			operation := "switch"
			if g.SelectedBranchIsTesting {
				operation = "test"
			}
			dpl := Deployment{
				UUID:       uuid.NewString(),
				Generation: *g,
				Operation:  operation,
				StartedAt:  time.Now().UTC(),
				Status:     Running,
			}
			d.mu.Lock()
			d.previousDeployment = d.Deployment
			d.Deployment = &dpl
			d.IsDeploying = true
			d.mu.Unlock()

			ctx := context.TODO()
			cominNeedRestart, profilePath, err := d.deployerFunc(
				ctx,
				g.OutPath,
				operation,
			)

			d.mu.Lock()
			d.IsDeploying = false
			d.Deployment.EndedAt = time.Now().UTC()
			d.Deployment.Err = err
			if err != nil {
				d.Deployment.ErrorMsg = err.Error()
				d.Deployment.Status = Failed
			} else {
				d.Deployment.Status = Done
			}
			d.Deployment.RestartComin = cominNeedRestart
			d.Deployment.ProfilePath = profilePath
			d.DeploymentDoneCh <- *d.Deployment
			d.mu.Unlock()

			cmd := d.postDeploymentCommand
			if cmd != "" {
				_, err = runPostDeploymentCommand(cmd, d.Deployment)
				if err != nil {
					logrus.Errorf("deployer: deploying generation %s, post deployment command [%s] failed %v", g.UUID, cmd, err)
				}
			}
		}
	}()
}
