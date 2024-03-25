package generation

import (
	"context"
	"fmt"
	"time"

	"github.com/nlewo/comin/internal/repository"
	"github.com/sirupsen/logrus"
)

type Status int64

const (
	Init Status = iota
	Evaluating
	Evaluated
	Building
	Built
)

// We consider each created genration is legit to be deployed: hard
// reset is ensured at RepositoryStatus creation.
type Generation struct {
	flakeUrl  string
	hostname  string
	machineId string

	Status Status

	RepositoryStatus repository.RepositoryStatus

	EvalStartedAt time.Time
	evalTimeout   time.Duration
	evalFunc      EvalFunc
	evalCh        chan EvalResult

	EvalEndedAt   time.Time
	EvalErr       error
	OutPath       string
	DrvPath       string
	EvalMachineId string

	BuildStartedAt time.Time
	BuildEndedAt   time.Time
	BuildErr       error
	buildFunc      BuildFunc
	buildCh        chan BuildResult
}

type EvalFunc func(ctx context.Context, flakeUrl string, hostname string) (drvPath string, outPath string, machineId string, err error)
type BuildFunc func(ctx context.Context, drvPath string) error

type BuildResult struct {
	EndAt time.Time
	Err   error
}

type EvalResult struct {
	EndAt     time.Time
	OutPath   string
	DrvPath   string
	MachineId string
	Err       error
}

func New(repositoryStatus repository.RepositoryStatus, flakeUrl, hostname, machineId string, evalFunc EvalFunc, buildFunc BuildFunc) Generation {
	return Generation{
		RepositoryStatus: repositoryStatus,
		evalTimeout:      6 * time.Second,
		evalFunc:         evalFunc,
		buildFunc:        buildFunc,
		flakeUrl:         flakeUrl,
		hostname:         hostname,
		machineId:        machineId,
		Status:           Init,
	}
}

func (g Generation) EvalCh() chan EvalResult {
	return g.evalCh
}

func (g Generation) BuildCh() chan BuildResult {
	return g.buildCh
}

func (g Generation) UpdateEval(r EvalResult) Generation {
	logrus.Debugf("Eval done with %#v", r)
	g.EvalEndedAt = r.EndAt
	g.DrvPath = r.DrvPath
	g.OutPath = r.OutPath
	g.EvalErr = r.Err
	g.EvalMachineId = r.MachineId
	g.Status = Evaluated
	return g
}

func (g Generation) UpdateBuild(r BuildResult) Generation {
	logrus.Debugf("Build done with %#v", r)
	g.BuildEndedAt = r.EndAt
	g.BuildErr = r.Err
	g.Status = Built
	return g
}

func (g Generation) Eval(ctx context.Context) Generation {
	g.evalCh = make(chan EvalResult)
	g.EvalStartedAt = time.Now()
	g.Status = Evaluating

	fn := func() {
		ctx, cancel := context.WithTimeout(ctx, g.evalTimeout)
		defer cancel()
		drvPath, outPath, machineId, err := g.evalFunc(ctx, g.flakeUrl, g.hostname)
		evaluationResult := EvalResult{
			EndAt: time.Now(),
		}
		if err == nil {
			evaluationResult.DrvPath = drvPath
			evaluationResult.OutPath = outPath
			if machineId != "" && g.machineId != machineId {
				evaluationResult.Err = fmt.Errorf("The evaluated comin.machineId '%s' is different from the /etc/machine-id '%s' of this machine",
					g.machineId, machineId)
			}
		} else {
			evaluationResult.Err = err
		}
		g.evalCh <- evaluationResult
	}
	go fn()
	return g
}

func (g Generation) Build(ctx context.Context) Generation {
	g.buildCh = make(chan BuildResult)
	g.BuildStartedAt = time.Now()
	g.Status = Building
	fn := func() {
		ctx, cancel := context.WithTimeout(ctx, g.evalTimeout)
		defer cancel()
		err := g.buildFunc(ctx, g.DrvPath)
		buildResult := BuildResult{
			EndAt: time.Now(),
		}
		buildResult.Err = err
		g.buildCh <- buildResult
	}
	go fn()
	return g
}
