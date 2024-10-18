package deployment

import (
	"context"
	"time"

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

func StatusFromString(status string) Status {
	switch status {
	case "init":
		return Init
	case "running":
		return Running
	case "done":
		return Done
	case "failed":
		return Failed
	}
	return Init
}

type DeployFunc func(context.Context, string, string, string) (bool, string, error)

type Deployment struct {
	UUID       string             `json:"uuid"`
	Generation builder.Generation `json:"generation"`
	StartAt    time.Time          `json:"start_at"`
	EndAt      time.Time          `json:"end_at"`
	// It is ignored in the JSON marshaling
	Err          error  `json:"-"`
	ErrorMsg     string `json:"error_msg"`
	RestartComin bool   `json:"restart_comin"`
	ProfilePath  string `json:"profile_path"`
	Status       Status `json:"status"`
	Operation    string `json:"operation"`

	deployerFunc DeployFunc
	deploymentCh chan DeploymentResult
}

type DeploymentResult struct {
	Err          error
	EndAt        time.Time
	RestartComin bool
	ProfilePath  string
}

func New(g builder.Generation, deployerFunc DeployFunc, deploymentCh chan DeploymentResult) Deployment {
	operation := "switch"
	if g.SelectedBranchIsTesting {
		operation = "test"
	}

	return Deployment{
		UUID:         uuid.NewString(),
		Generation:   g,
		deployerFunc: deployerFunc,
		deploymentCh: deploymentCh,
		Status:       Init,
		Operation:    operation,
	}
}

func (d Deployment) Update(dr DeploymentResult) Deployment {
	d.EndAt = dr.EndAt
	d.Err = dr.Err
	if d.Err != nil {
		d.ErrorMsg = dr.Err.Error()
	}
	d.RestartComin = dr.RestartComin
	d.ProfilePath = dr.ProfilePath
	if dr.Err == nil {
		d.Status = Done
	} else {
		d.Status = Failed
	}
	return d
}

func (d Deployment) IsTesting() bool {
	return d.Operation == "test"
}

// Deploy returns a updated deployment (mainly the startAt is updated)
// and asyncronously tun the deployment. Once finished, a
// DeploymentResult is emitted on the channel d.deploymentCh.
func (d Deployment) Deploy(ctx context.Context) Deployment {
	go func() {
		// FIXME: propagate context
		cominNeedRestart, profilePath, err := d.deployerFunc(
			ctx,
			d.Generation.MachineId,
			d.Generation.OutPath,
			d.Operation,
		)

		deploymentResult := DeploymentResult{}
		deploymentResult.Err = err
		if err != nil {
			logrus.Error(err)
			logrus.Infof("Deployment failed")
		}

		deploymentResult.EndAt = time.Now().UTC()
		deploymentResult.RestartComin = cominNeedRestart
		deploymentResult.ProfilePath = profilePath
		d.deploymentCh <- deploymentResult
	}()
	d.Status = Running
	d.StartAt = time.Now().UTC()
	return d
}
