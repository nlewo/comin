package deployment

import (
	"context"
	"time"

	"github.com/nlewo/comin/internal/generation"
	"github.com/sirupsen/logrus"
)

type Status int64

const (
	Init Status = iota
	Running
	Done
	Failed
)

type DeployFunc func(context.Context, string, string, string) (bool, error)

type Deployment struct {
	Generation generation.Generation `json:"generation"`
	StartAt    time.Time             `json:"start_at"`
	EndAt      time.Time             `json:"end_at"`
	// It is ignored in the JSON marshaling
	Err          error  `json:"-"`
	ErrorMsg     string `json:"error_msg"`
	RestartComin bool   `json:"restart_comin"`
	Status       Status `json:"status"`

	deployerFunc DeployFunc
	deploymentCh chan DeploymentResult
}

type DeploymentResult struct {
	Err          error
	EndAt        time.Time
	RestartComin bool
}

func New(g generation.Generation, deployerFunc DeployFunc, deploymentCh chan DeploymentResult) Deployment {
	return Deployment{
		Generation:   g,
		deployerFunc: deployerFunc,
		deploymentCh: deploymentCh,
		Status:       Init,
	}
}

func (d Deployment) Update(dr DeploymentResult) Deployment {
	d.EndAt = dr.EndAt
	d.Err = dr.Err
	if d.Err != nil {
		d.ErrorMsg = dr.Err.Error()
	}
	d.RestartComin = dr.RestartComin
	if dr.Err == nil {
		d.Status = Done
	} else {
		d.Status = Failed
	}
	return d
}

// Deploy returns a updated deployment (mainly the startAt is updated)
// and asyncronously tun the deployment. Once finished, a
// DeploymentResult is emitted on the channel d.deploymentCh.
func (d Deployment) Deploy(ctx context.Context) Deployment {
	go func() {

		operation := "switch"
		if d.Generation.RepositoryStatus.IsTesting() {
			operation = "test"
		}

		logrus.Debugf("The operation is %s", operation)

		// FIXME: propagate context
		cominNeedRestart, err := d.deployerFunc(
			ctx,
			d.Generation.EvalMachineId,
			d.Generation.OutPath,
			operation,
		)

		deploymentResult := DeploymentResult{}
		deploymentResult.Err = err
		if err != nil {
			logrus.Error(err)
			logrus.Infof("Deployment failed")
		}

		deploymentResult.EndAt = time.Now()
		deploymentResult.RestartComin = cominNeedRestart
		d.deploymentCh <- deploymentResult
	}()
	d.Status = Running
	d.StartAt = time.Now()
	return d
}
